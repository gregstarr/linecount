package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

var (
	nonTextError = errors.New("file not UTF-8")
)

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		if !utf8.Valid(buf[:c]) {
			return count, nonTextError
		}
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

func poolFunc(fi interface{}) {
	file := fi.(string)
	f, err := os.Open(file)
	if err != nil {
		log.Println(err)
		return
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	count, err := lineCounter(f)
	if err != nil {
		log.Printf("%s: %s", err, filepath.Base(file))
		return
	}
	atomic.AddInt64(&total, int64(count))
}

func runNonRecursive() {
	var files []string
	var err error
	if glob {
		files, err = filepath.Glob(input)
		if err != nil {
			panic(err)
		}
	} else {
		dirEntry, err := os.ReadDir(input)
		if err != nil {
			panic(err)
		}
		files = make([]string, 0)
		for _, f := range dirEntry {
			if f.IsDir() {
				continue
			}
			files = append(files, filepath.Join(input, f.Name()))
		}
	}
	fmt.Println("num files:", len(files))
	p, err := ants.NewPoolWithFunc(100, func(file interface{}) {
		poolFunc(file)
		wg.Done()
	})
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		wg.Add(1)
		_ = p.Invoke(file)
	}
	wg.Wait()
	fmt.Println("total lines:", total)
}

func runRecursive() {
	fmt.Println("recursive search")
	p, err := ants.NewPoolWithFunc(100, func(file interface{}) {
		poolFunc(file)
		wg.Done()
	})
	if err != nil {
		panic(err)
	}
	err = filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if d.IsDir() {
			fmt.Println(path)
			return nil
		}
		wg.Add(1)
		_ = p.Invoke(path)
		return nil
	})
	wg.Wait()
	fmt.Println("total lines:", total)
}

func parseCommand() {
	flag.BoolVar(&recursive, "r", false, "recursive search for files")
	flag.BoolVar(&glob, "g", false, "glob pattern")
	flag.Parse()
	input = flag.Arg(0)
	if input == "" {
		fmt.Println("linecount")
		fmt.Println("	count lines in files")
		fmt.Println()
		fmt.Println("arguments:")
		flag.PrintDefaults()
		os.Exit(0)
	}
}

var (
	recursive bool
	glob      bool
	input     string
	total     int64
	wg        sync.WaitGroup
)

func main() {
	defer ants.Release()
	parseCommand()
	if recursive {
		runRecursive()
	} else {
		runNonRecursive()
	}
}
