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

	err := srv.fetchStdlibImages()
	if err != nil {
		log.Fatal(err)
	}

	go srv.fetchImages()

	log.Printf("ji-marina listening on %q...\n", srv.addr)
	log.Fatal(http.ListenAndServe(srv.addr, mux))
}

type server struct {
	addr string

	mu  sync.RWMutex
	cli *client.Client
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
	fmt.Fprintf(w, "<h1>Welcome to the Marina</h1>\n")
	srv.list(w)
}

func (srv *server) fetchImages() {
	ticker := time.NewTicker(1 * time.Hour)
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
	imgs, err := srv.fetchRI3ImageList()
	if err != nil {
		return err
	}

	start := time.Now()
	for _, img := range imgs {
		err := srv.pullImage(img)
		if err != nil {
			return err
		}
	}
	log.Printf("pulled %d images in %v\n", len(imgs), time.Since(start))
	return nil
}

func (srv *server) fetchStdlibImages() error {
	stdlib := []string{
		"alpine:latest",
		"busybox:latest",
		"debian:latest",
		"centos:latest",
		"fedora:latest",
		"golang:latest",
		"python:latest",
		"ubuntu:latest",
	}

	start := time.Now()
	log.Printf("pulling stdlib images...\n")
	for _, name := range stdlib {
		err := srv.pullImage(name)
		if err != nil {
			return err
		}
	}
	log.Printf("pulling stdlib images... [done] (%v)\n", time.Since(start))
	return nil
}

func (srv *server) fetchRI3ImageList() ([]string, error) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	ctx := context.Background()
	opts := types.ImageSearchOptions{Limit: 100}
	imgs, err := srv.cli.ImageSearch(ctx, imagePrefix+"/*", opts)
	if err != nil {
		return nil, err
	}

	images := make([]string, len(imgs))
	for i, img := range imgs {
		images[i] = img.Name
	}
	return images, nil
}

func (srv *server) pullImage(name string) error {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	ctx := context.Background()
	start := time.Now()
	log.Printf("pulling %v...\n", name)
	opts := types.ImagePullOptions{}
	if strings.HasPrefix(name, imagePrefix+"/") {
		opts.All = true
	}
	r, err := srv.cli.ImagePull(ctx, name, opts)
	if err != nil {
		log.Printf("image-pull error %q: %v\n", name, err)
		return err
	}
	defer r.Close()

	const quiet = false
	load, err := srv.cli.ImageLoad(ctx, r, quiet)
	if err != nil {
		log.Printf("image-load error %q: %v\n", name, err)
		return err
	}
	defer load.Body.Close()
	log.Printf("pulling %v... [done] (%v)\n", name, time.Since(start))
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

	switch len(list) {
	case 0:
		log.Printf("no such image %q", name)
		http.Error(w, "no such image ["+name+"]", http.StatusBadRequest)
		return
	case 1:
		// ok
	default:
		log.Printf("more than 1 image found for %q (n=%d)\n", name, len(list))
		http.Error(w, "more than 1 image found for ["+name+"]", http.StatusBadRequest)
		return
	}

	img, err := srv.cli.ImageSave(ctx, []string{name})
	if err != nil {
		log.Printf("image-save %q: %v\n", name, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer img.Close()

	fname := name + ".tar"
	fname = strings.Replace(fname, "/", "-", -1)
	fname = strings.Replace(fname, ":", "-", -1)
	w.Header().Set("Content-Disposition", "attachment; filename="+fname)
	w.Header().Set("Content-Type", "application/x-tar")

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

	images, err := srv.listImages()

	fmt.Fprintf(w, "<h2>Docker images (total=%d)</h2>\n", len(images))
	if err != nil {
		log.Printf("<h3>Error retrieving image list: <p>%v</p></h3>\n", err)
		return
	}
	fmt.Fprintf(w, "<pre>\n")
	for _, img := range images {
		if strings.HasPrefix(img.Name, imagePrefix+"/") {
			fmt.Fprintf(w, " %-12s   %-50s (%8.3f MB) <a href=\"/docker-images/%s\">Download</a>\n", img.ID[:12], img.Name, float64(img.Size)/1024/1024, img.Name)
		}
	}
	fmt.Fprintf(w, "\n")
	for _, img := range images {
		if !strings.HasPrefix(img.Name, imagePrefix+"/") {
			fmt.Fprintf(w, " %-12s   %-50s (%8.3f MB) <a href=\"/docker-images/%s\">Download</a>\n", img.ID[:12], img.Name, float64(img.Size)/1024/1024, img.Name)
		}
	}
	fmt.Fprintf(w, "</pre>\n")
}

func (srv *server) listImages() ([]dkrImage, error) {
	srv.mu.RLock()
	defer srv.mu.RUnlock()

	ctx := context.Background()
	opts := types.ImageListOptions{All: true}
	imgs, err := srv.cli.ImageList(ctx, opts)
	if err != nil {
		return nil, err
	}

	var images []dkrImage
	for _, img := range imgs {
		id := img.ID[strings.Index(img.ID, ":")+1:]
		for _, tag := range img.RepoTags {
			if tag == "<none>:<none>" || tag == "" {
				continue
			}
			images = append(images, dkrImage{
				Name: tag,
				ID:   id,
				Size: img.VirtualSize,
			})
		}
	}

	sort.Sort(dkrImages(images))
	return images, nil
}

type dkrImage struct {
	Name string
	ID   string
	Size int64
}

type dkrImages []dkrImage

func (p dkrImages) Len() int           { return len(p) }
func (p dkrImages) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p dkrImages) Less(i, j int) bool { return p[i].Name < p[j].Name }
