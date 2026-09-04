package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const (
	docsPath   = "docs"
	tagsPrefix = "<!-- tags:"
	tagsSuffix = "-->"
	itemEnd    = "<!-- knowledge-item-end -->"
)

var ErrChunkTooLarge = errors.New("knowledge chunk exceeds appendix size limit")

type SearchArguments struct {
	Keywords []string `json:"keywords"`
}

type KnowledgeChunk struct {
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
	Size    uint64
}

type Knowledge struct {
	Content         []KnowledgeChunk
	Appendix        []KnowledgeChunk
	MaxAppendixSize uint64
	AppendixSize    uint64
}

func splitIntoChunks(data string) ([]KnowledgeChunk, error) {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")

	parts := strings.Split(data, itemEnd)

	if tail := parts[len(parts)-1]; strings.TrimSpace(tail) != "" {
		return nil, fmt.Errorf("knowledge item has no closing marker")
	}

	chunks := make([]KnowledgeChunk, 0, len(parts)-1)

	for i, part := range parts[:len(parts)-1] {
		part = strings.TrimLeft(part, "\n")

		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("knowledge item %d is empty", i+1)
		}

		lineEnd := strings.IndexByte(part, '\n')
		if lineEnd == -1 {
			return nil, fmt.Errorf(
				"knowledge item %d has no content line",
				i+1,
			)
		}

		header := strings.TrimSpace(part[:lineEnd])
		content := part[lineEnd+1:]

		content = strings.TrimSuffix(content, "\n")

		tags, err := parseTags(header)
		if err != nil {
			return nil, fmt.Errorf(
				"knowledge item %d: %w",
				i+1,
				err,
			)
		}

		chunk := KnowledgeChunk{
			Tags:    tags,
			Content: content,
		}
		chunk.Size = uint64(chunkSize(chunk))

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

func parseTags(header string) ([]string, error) {
	if !strings.HasPrefix(header, tagsPrefix) ||
		!strings.HasSuffix(header, tagsSuffix) {
		return nil, fmt.Errorf("invalid tags header %q", header)
	}

	value := strings.TrimPrefix(header, tagsPrefix)
	value = strings.TrimSuffix(value, tagsSuffix)
	value = strings.TrimSpace(value)

	if value == "" {
		return []string{}, nil
	}

	rawTags := strings.Split(value, ",")
	tags := make([]string, 0, len(rawTags))

	for _, rawTag := range rawTags {
		tag := strings.TrimSpace(rawTag)

		if tag == "" {
			return nil, fmt.Errorf("tags header contains an empty tag")
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

func (base *Knowledge) Load() error {
	entries, err := os.ReadDir(docsPath)

	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if entry.Name() == "__appendix.md" {
				content, err := os.ReadFile(fmt.Sprintf("%s/%s", docsPath, entry.Name()))

				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to load %s: %s", entry.Name(), err)
					continue
				}

				res, err := splitIntoChunks(string(content))

				if err != nil {
					return err
				}
				for _, chunk := range res {
					base.Append(chunk.Tags, chunk.Content)
				}
			} else if strings.HasSuffix(entry.Name(), ".md") {
				content, err := os.ReadFile(fmt.Sprintf("%s/%s", docsPath, entry.Name()))

				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to load %s: %s", entry.Name(), err)
					continue
				}

				res, err := splitIntoChunks(string(content))
				if err != nil {
					return err
				}

				base.Content = append(base.Content, res...)
			}
		}
	}

	return nil
}

func (base *Knowledge) Dump() error {
	var fileData strings.Builder

	for _, item := range base.Appendix {
		fileData.WriteString("<!-- tags: ")
		fileData.WriteString(strings.Join(item.Tags, ", "))
		fileData.WriteString(" -->\n")

		fileData.WriteString(strings.TrimRight(item.Content, "\n"))
		fileData.WriteString("\n<!-- knowledge-item-end -->\n")
	}

	return os.WriteFile(
		filepath.Join(docsPath, "__appendix.md"),
		[]byte(fileData.String()),
		0644,
	)
}

func (base *Knowledge) Append(tags []string, content string) error {
	if base.MaxAppendixSize == 0 {
		return nil
	}

	chunk := KnowledgeChunk{
		Tags:    append([]string(nil), tags...),
		Content: content,
	}

	chunk.Size = uint64(chunkSize(chunk))

	if chunk.Size > base.MaxAppendixSize {
		return ErrChunkTooLarge
	}

	base.Appendix = append(base.Appendix, chunk)
	base.AppendixSize += chunk.Size

	for base.AppendixSize > base.MaxAppendixSize {
		first := base.Appendix[0]

		base.AppendixSize -= first.Size
		clear(base.Appendix[:1])
		base.Appendix = base.Appendix[1:]
	}

	return nil
}

func chunkSize(chunk KnowledgeChunk) int {
	var data strings.Builder

	data.WriteString("<!-- tags: ")
	data.WriteString(strings.Join(chunk.Tags, ", "))
	data.WriteString(" -->\n")
	data.WriteString(strings.TrimRight(chunk.Content, "\n"))
	data.WriteString("\n<!-- knowledge-item-end -->\n")

	return data.Len()
}

func (base *Knowledge) Search(keywords []string, searchAmount int) []string {
	if searchAmount <= 0 || len(keywords) == 0 {
		return []string{}
	}

	normalizedKeywords := make(map[string]struct{}, len(keywords))

	for _, keyword := range keywords {
		keyword = normalizeKeyword(keyword)

		if keyword != "" {
			normalizedKeywords[keyword] = struct{}{}
		}
	}

	if len(normalizedKeywords) == 0 {
		return []string{}
	}

	type searchResult struct {
		Content string
		Score   int
	}

	chunks := make([]KnowledgeChunk, 0, len(base.Content)+len(base.Appendix))
	chunks = append(chunks, base.Content...)
	chunks = append(chunks, base.Appendix...)

	results := make([]searchResult, 0, len(chunks))

	for _, chunk := range chunks {
		score := calculateChunkScore(chunk, normalizedKeywords)

		if score == 0 {
			continue
		}

		results = append(results, searchResult{
			Content: chunk.Content,
			Score:   score,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if searchAmount > len(results) {
		searchAmount = len(results)
	}

	content := make([]string, searchAmount)

	for i := range searchAmount {
		content[i] = results[i].Content
	}

	return content
}

func calculateChunkScore(
	chunk KnowledgeChunk,
	keywords map[string]struct{},
) int {
	const (
		tagMatchScore     = 10
		contentMatchScore = 1
	)

	tags := make(map[string]struct{}, len(chunk.Tags))

	for _, tag := range chunk.Tags {
		tag = normalizeKeyword(tag)

		if tag != "" {
			tags[tag] = struct{}{}
		}
	}

	words := extractWords(chunk.Content)

	score := 0

	for keyword := range keywords {
		if _, found := tags[keyword]; found {
			score += tagMatchScore
		}

		if _, found := words[keyword]; found {
			score += contentMatchScore
		}
	}

	return score
}

func extractWords(content string) map[string]struct{} {
	words := strings.FieldsFunc(
		strings.ToLower(content),
		func(char rune) bool {
			return !unicode.IsLetter(char) &&
				!unicode.IsNumber(char) &&
				char != '_'
		},
	)

	result := make(map[string]struct{}, len(words))

	for _, word := range words {
		result[word] = struct{}{}
	}

	return result
}

func normalizeKeyword(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}
