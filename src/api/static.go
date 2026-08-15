package api

import (
	"net/http"
	"path"
)

type SPAServer struct {
	rootDir http.FileSystem
}

func (spa *SPAServer) Serve(w http.ResponseWriter, r *http.Request) {
	p := path.Clean(r.URL.Path)

	f, err := spa.rootDir.Open(p)
	if err == nil {
		info, statErr := f.Stat()
		f.Close()

		if statErr == nil && !info.IsDir() {
			http.FileServer(spa.rootDir).ServeHTTP(w, r)
			return
		}
	}

	r.URL.Path = "/index.html"
	http.FileServer(spa.rootDir).ServeHTTP(w, r)
}
