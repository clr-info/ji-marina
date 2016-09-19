// Copyright ©2016 The ji-marina Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const imagePrefix = "piscineri3"

func main() {
	addr := flag.String("addr", ":80", "web server address")
	flag.Parse()

	srv := newServer(*addr)

	mux := http.NewServeMux()
	mux.Handle("/", srv)
	mux.HandleFunc("/docker-images/", srv.image)
	mux.HandleFunc("/docker-update", srv.update)

	go srv.fetchImages()

	log.Printf("ji-marina listening on %q...\n", srv.addr)
	err := http.ListenAndServe(srv.addr, mux)
	if err != nil {
		log.Fatal(err)
	}
}

type server struct {
	addr string

	mu     sync.RWMutex
	images []string
	cli    *client.Client
}

func newServer(addr string) *server {
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "v1.22", nil, defaultHeaders)
	if err != nil {
		log.Fatal(err)
	}

	return &server{
		addr: addr,
		cli:  cli,
	}
}

func (srv *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Welcome to the Piscine</h1>\n")

	srv.list(w)
}

func (srv *server) fetchImages() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	err := srv.pull()
	if err != nil {
		log.Printf("error: %v\n", err)
	}

	for {
		select {
		case <-ticker.C:
			err := srv.pull()
			if err != nil {
				log.Printf("error: %v\n", err)
			}
		}
	}
}

func (srv *server) pull() error {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	ctx := context.Background()

	opts := types.ImageSearchOptions{Limit: 100}
	imgs, err := srv.cli.ImageSearch(ctx, imagePrefix+"/*", opts)
	if err != nil {
		return err
	}

	start := time.Now()
	var images []string
	for _, img := range imgs {
		imgStart := time.Now()
		log.Printf("pulling %v...\n", img.Name)
		opts := types.ImagePullOptions{All: true}
		r, err := srv.cli.ImagePull(ctx, img.Name, opts)
		if err != nil {
			log.Printf("image-pull error: %v\n", err)
			return err
		}
		defer r.Close()

		const quiet = false
		load, err := srv.cli.ImageLoad(ctx, r, quiet)
		if err != nil {
			log.Printf("image-load error: %v\n", err)
			return err
		}
		defer load.Body.Close()
		log.Printf("pulling %v... [done] (%v)\n", img.Name, time.Since(imgStart))
		images = append(images, img.Name)
	}
	srv.images = images
	log.Printf("pulled %d images in %v\n", len(images), time.Since(start))
	return nil
}

func (srv *server) image(w http.ResponseWriter, r *http.Request) {
	const hdr = "/docker-images/"
	name := r.URL.Path[len(hdr):]
	log.Printf("image: %v\n", name)

	switch strings.Count(name, ":") {
	case 0:
		name += ":latest"
	case 1:
		// ok
	default:
		log.Printf("invalid image name %q\n", name)
		http.Error(w, "invalid image name ["+name+"]", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	opts := types.ImageListOptions{All: true, MatchName: name}
	list, err := srv.cli.ImageList(ctx, opts)
	if err != nil {
		log.Printf("image-list %q: %v\n", name, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(list) != 1 {
		log.Printf("didn't get exactly 1 image (n=%d)\n", len(list))
		http.Error(w, "not 1 image found", http.StatusBadRequest)
		return
	}

	img, err := srv.cli.ImageSave(ctx, []string{name})
	if err != nil {
		log.Printf("image-save %q: %v\n", name, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer img.Close()

	_, err = io.Copy(w, img)
	if err != nil {
		log.Printf("image-copy %q: %v\n", name, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (srv *server) update(w http.ResponseWriter, r *http.Request) {
	err := srv.pull()
	if err != nil {
		fmt.Fprintf(w, "Error: %v\n", err)
		return
	}

	srv.list(w)
}

func (srv *server) list(w io.Writer) {
	srv.mu.RLock()
	defer srv.mu.RUnlock()

	fmt.Fprintf(w, "<h2>Docker images (total=%d)</h2>\n", len(srv.images))
	fmt.Fprintf(w, "<ul>\n")
	sort.Strings(srv.images)
	for _, img := range srv.images {
		fmt.Fprintf(w, "\t<li>%s</li>\n", img)
	}
	fmt.Fprintf(w, "</ul>\n")
}
