package main

import (
	"encoding/json"
	"fmt"
	"time"

	sdk "tripod311/familiar-sdk"
)

var module *sdk.ExternalModule

var DateTimeTool = sdk.ToolDescription{
	Name:        "system_datetime",
	Description: "Returns the current local date, time, timezone, UTC offset, and Unix timestamp. Use this when the current date or time is needed, including resolving relative dates such as today, yesterday, or tomorrow.",
	Parameters: json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`),
}

type DateTimeResult struct {
	Date       string `json:"date"`
	Time       string `json:"time"`
	DateTime   string `json:"datetime"`
	Timezone   string `json:"timezone"`
	UTCOffset  string `json:"utcOffset"`
	Unix       int64  `json:"unix"`
	UnixMillis int64  `json:"unixMillis"`
}

func main() {
	module = sdk.NewExternalModule()

	module.GatherFunctions = GatherFunctions
	module.CallFunction = CallFunction

	module.Start()
}

func GatherFunctions() ([]sdk.ToolDescription, error) {
	return []sdk.ToolDescription{
		DateTimeTool,
	}, nil
}

func CallFunction(name string, arguments string) (json.RawMessage, error) {
	switch name {
	case "system_datetime":
		now := time.Now()

		_, offsetSeconds := now.Zone()

		result := DateTimeResult{
			Date:       now.Format("2006-01-02"),
			Time:       now.Format("15:04:05"),
			DateTime:   now.Format(time.RFC3339),
			Timezone:   now.Location().String(),
			UTCOffset:  formatUTCOffset(offsetSeconds),
			Unix:       now.Unix(),
			UnixMillis: now.UnixMilli(),
		}

		resBytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("serialization error: %w", err)
		}

		return resBytes, nil

	default:
		return nil, fmt.Errorf("unknown function: %s", name)
	}
}

func formatUTCOffset(seconds int) string {
	sign := "+"

	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	return fmt.Sprintf(
		"%s%02d:%02d",
		sign,
		hours,
		minutes,
	)
}
