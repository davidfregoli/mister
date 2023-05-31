package main

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	file, err := os.Open("./books")
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(file)
	var wg = sync.WaitGroup{}
	os.Mkdir("/files/input", os.ModePerm)
	os.Mkdir("/files/intermediate", os.ModePerm)
	os.Mkdir("/files/output", os.ModePerm)
	for scanner.Scan() {
		wg.Add(1)
		line := strings.Split(scanner.Text(), ";")
		book, url := line[0], line[1]
		go func(book string, url string) {
			cmd := exec.Command("curl", "-L", "-o", "/files/input/"+book+".txt", url)
			cmd.Run()
			wg.Done()
		}(book, url)
	}
	wg.Wait()
	file.Close()

	err = scanner.Err()
	if err != nil {
		log.Fatal(err)
	}
}
