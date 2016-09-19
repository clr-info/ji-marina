// Copyright ©2016 The ji-marina Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
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

	resp, err := http.Get("http://134.158.120.183/docker-images/" + name)
	if err != nil {
		log.Fatalf("marina-get %q: %v\n", name, err)
	}
	defer resp.Body.Close()

	load := exec.Command("docker", "load")
	load.Stdin = resp.Body
	load.Stdout = os.Stdout
	load.Stderr = os.Stderr
	err = load.Run()
	if err != nil {
		log.Fatalf("docker-load %q: %v\n", name, err)
	}
}
