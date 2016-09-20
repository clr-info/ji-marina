// Copyright ©2016 The ji-marina Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	run()
}

func run() {
	name := os.Args[1]

	start := time.Now()
	defer func() {
		log.Printf("pulling %q... [done] (%v)", name, time.Since(start))
	}()
	log.Printf("pulling %q...\n", name)

	resp, err := http.Get("http://piscine.in2p3.fr:8080/docker-images/" + name)
	if err != nil {
		log.Fatalf("marina-get %q: %v\n", name, err)
	}
	defer resp.Body.Close()

	load := exec.Command("docker", "load")
	load.Stdin = resp.Body
	stdout, err := load.StdoutPipe()
	if err != nil {
		log.Fatalf("docker-load-stdout %q: %v\n", name, err)
	}

	stderr, err := load.StderrPipe()
	if err != nil {
		log.Fatalf("docker-load-stderr %q: %v\n", name, err)
	}

	err = load.Start()
	if err != nil {
		log.Fatalf("docker-load-start %q: %v\n", name, err)
		return
	}

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	err = load.Wait()
	if err != nil {
		log.Fatalf("docker-load %q: %v\n", name, err)
	}
}
