package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/restic/restic/chunker"
)

func isFile(fi os.FileInfo) bool {
	return fi.Mode()&(os.ModeType|os.ModeCharDevice) == 0
}

func chunkify(m map[string]uint, filename string) (total uint) {
	f, err := os.Open(filename)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return 0
	}
	defer f.Close()

	ch := chunker.New(f, 512*chunker.KiB, sha256.New())

	for {
		chunk, err := ch.Next()

		if err == io.EOF {
			return
		}

		if err != nil {
			fmt.Printf("err: %v\n", err)
			return
		}

		// fmt.Printf("chunk: %v\n", chunk.Length)
		m[hex.EncodeToString(chunk.Digest)] = chunk.Length
		total += chunk.Length
	}
}

var tests = []struct {
	bits uint
	win  uint
	min  uint
}{
	{16, 16, 1024},
	{16, 16, 8192},
	{16, 16, 65536},
	{16, 128, 1024},
	{16, 128, 8192},
	{16, 128, 65536},
	{16, 1024, 1024},
	{16, 1024, 8192},
	{16, 1024, 65536},
	{16, 4095, 1024}, // attic 0.14
	{16, 4095, 8192},
	{16, 4095, 65536},
	{18, 16, 1024},
	{18, 16, 8192},
	{18, 16, 65536},
	{18, 128, 1024},
	{18, 128, 8192},
	{18, 128, 65536},
	{18, 1024, 1024},
	{18, 1024, 8192},
	{18, 1024, 65536},
	{18, 4095, 1024},
	{18, 4095, 8192},
	{18, 4095, 65536},
	{20, 16, 1024},
	{20, 16, 8192},
	{20, 16, 65536},
	{20, 16, 65536 * 8}, // restic
	{20, 128, 1024},
	{20, 128, 8192},
	{20, 128, 65536},
	{20, 1024, 1024},
	{20, 1024, 8192},
	{20, 1024, 65536},
	{20, 4095, 1024},
	{20, 4095, 8192},
	{20, 4095, 65536},
}

func run_test(bits, win, min uint) {
	chunker.UpdateSplitmask(bits)
	chunker.WindowSize = int(win)
	chunker.MinSize = min

	start := time.Now()

	chunks := make(map[string]uint)
	var total, files uint
	for _, dir := range os.Args[1:] {
		filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
			if isFile(fi) {
				files++
				bytes := chunkify(chunks, p)
				// fmt.Printf("chunkify %v: %v\n", p, bytes)
				total += bytes
			}
			return nil
		})
	}

	duration := float64(time.Since(start) / time.Second) // in seconds

	var dedup uint
	for _, v := range chunks {
		dedup += v
	}

	fmt.Printf("%d\t%d\t%d\t%.3f\t%.1f\t%d\t%d\t%.2f\t%.2f\n",
		bits, chunker.WindowSize, chunker.MinSize,
		float64(dedup)/float64(total), duration, len(chunks), files,
		float64(total)/chunker.GiB, float64(dedup)/chunker.GiB)
}

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "usage: chunker DIR [DIR]\n")
		os.Exit(1)
	}

	fmt.Printf("mask_bits\twindow_size\tchunk_min\tratio\truntime\tchunk_count\tfile_count\ttotal_size\ttotal_deduped_size\n")
	for _, test := range tests {
		run_test(test.bits, test.win, test.min)
	}
}
