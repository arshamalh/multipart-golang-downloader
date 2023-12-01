// Multipart (concurrent) downloader written in Go
// Ability to detect whether the file is downloadable concurrently or not
// Ability to show progress of a downloading file
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type downloader struct {
	client               *http.Client
	workersCount         int
	chunks               []bytes.Buffer
	progressChan         chan int
	progressEnabled      bool
	progressCalcInterval int
}

func main() {
	var progressEnabled bool
	var workersCount int
	var progressCalcInterval int

	var root = &cobra.Command{
		Use:   "downloader",
		Short: "CLI tool for downloading a file concurrently",
	}

	var cmd = &cobra.Command{
		Use:   "download [link]",
		Short: "downloading a file",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				log.Fatal("wrong number of arguments passed ", len(args))
			}
			if workersCount <= 0 {
				log.Fatal("workers count can't be less than 1, and 1 is used for non-concurrent mode")
			}
			// Not to fast to consume all the resources
			if progressCalcInterval < 50 {
				progressCalcInterval = 50
			}

			if err := run(workersCount, progressEnabled, progressCalcInterval, args[0]); err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.Flags().IntVarP(&workersCount, "workers-count", "w", 5, "number of workers (default is 5 and 1 can be used for non-concurrent code)")
	cmd.Flags().IntVarP(&progressCalcInterval, "progress-calc-interval", "i", 300, "the amount of time (in millisecond) in between of recalculating the progress of a downloading file")
	cmd.Flags().BoolVarP(&progressEnabled, "progress-enabled", "p", true, "show the progress or not (default is true)")

	root.AddCommand(cmd)
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(workersCount int, progressEnabled bool, progressCalcInterval int, link string) error {
	d := NewDownloader(workersCount)
	d.WithProgress(progressEnabled, progressCalcInterval)
	if progressEnabled {
		// Consume progress in a separate goroutine
		go func() {
			for progress := range d.ConsumeProgress() {
				fmt.Println(progress, "%", "downloaded")
			}
		}()
	}

	filePath, err := d.Download(link)
	if err != nil {
		return err
	}

	fmt.Println("file is successfully written to:", filePath)
	return nil
}

// IMPORTANT: use one downloader per download or lock users to download only one file at a time.
//
//	One downloader downloading multiple files will may have unexpected behavior.
//
// TODO: Calculate workers count dynamically and combine its logic with process single
func NewDownloader(workersCount int) *downloader {
	return &downloader{
		workersCount: workersCount,
		chunks:       make([]bytes.Buffer, workersCount),
		progressChan: make(chan int),
		client:       &http.Client{},
	}
}

func (d *downloader) WithCustomHttpClient(client *http.Client) {
	d.client = client
}

func (d *downloader) WithProgress(isEnabled bool, interval int) {
	d.progressEnabled = isEnabled
	d.progressCalcInterval = interval
}

// Downloads a file, store it in the file system and returns the path to the file,
// or raise an error if it can't download the file or can't store it.
func (d *downloader) Download(fileURL string) (string, error) {
	fmt.Println("downloading podcast", "url:", fileURL)
	isMultipartSupported, contentLength, err := d.getRangeDetails(fileURL)
	if err != nil {
		return "", err
	}

	if d.progressEnabled {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go d.progress(ctx, contentLength)
	}

	if isMultipartSupported && d.workersCount > 1 {
		return d.processMultiple(contentLength, fileURL)
	}

	return d.processSingle(fileURL)
}

// Returns a channel returning numerical values between 0 and 100 representing the percentage of file downloaded.
func (d *downloader) ConsumeProgress() <-chan int {
	return d.progressChan
}

func (d *downloader) processSingle(url string) (filePath string, err error) {
	fmt.Println("processing single")
	d.chunks[0] = bytes.Buffer{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	response, err := d.client.Do(request)
	if err != nil {
		fmt.Println(err)
	}
	defer response.Body.Close()

	fmt.Println("started writing to buffer")
	written, err := io.Copy(&d.chunks[0], response.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("written %d bytes to the buffer\n", written)

	return d.combineChunks(url)
}

func (d *downloader) processMultiple(contentLength int, url string) (filePath string, err error) {
	fmt.Println("processing multiple")
	partLength := contentLength / d.workersCount
	var wg sync.WaitGroup
	wg.Add(d.workersCount)

	for startRange, index := 0, 0; startRange < contentLength; startRange += partLength + 1 {
		endRange := startRange + partLength
		if endRange > contentLength {
			endRange = contentLength
		}
		_range := fmt.Sprintf("%d-%d", startRange, endRange)
		go d.downloadFileForRange(&wg, url, _range, index)
		index++
	}

	wg.Wait()

	if err != nil {
		return "", err
	}

	return d.combineChunks(url)
}

func (d *downloader) downloadFileForRange(wg *sync.WaitGroup, url, _range string, index int) {
	defer wg.Done()
	fmt.Printf("range %s started\n", _range)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	request.Header.Add("Range", "bytes="+_range)

	response, err := d.client.Do(request)
	if err != nil {
		fmt.Println(err)
	}
	defer response.Body.Close()

	fmt.Println("started writing to buffer")
	d.chunks[index] = bytes.Buffer{}
	written, err := io.Copy(&d.chunks[index], response.Body)
	fmt.Println(written, err)
}

func (d *downloader) combineChunks(url string) (filePath string, err error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	filePath = path.Join(currentDir, "/", filepath.Base(url))
	output, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer output.Close()

	for i := 0; i < len(d.chunks); i++ {
		if _, err = d.chunks[i].WriteTo(output); err != nil {
			return "", err
		}
	}

	return filePath, nil
}

func (d *downloader) progress(ctx context.Context, totalLen int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			totalDownloaded := 0
			for _, chunk := range d.chunks {
				totalDownloaded += int((float32(chunk.Len()) / float32(totalLen)) * 100)
			}
			if totalDownloaded > 100 {
				totalDownloaded = 100
			}
			d.progressChan <- totalDownloaded
		}
		time.Sleep(time.Millisecond * time.Duration(d.progressCalcInterval))
	}
}

func (d *downloader) getRangeDetails(url string) (bool, int, error) {
	response, err := d.client.Head(url)

	if err != nil {
		// If resets by peer, we should tell user that we don't support downloading this podcast
		return false, 0, err
	}

	if response.StatusCode != 200 && response.StatusCode != 206 {
		return false, 0, err
	}

	contentLength, err := strconv.Atoi(response.Header.Get("Content-Length"))
	if err != nil {
		return false, 0, err
	}

	if response.Header.Get("Accept-Ranges") == "bytes" {
		return true, contentLength, nil
	}

	return false, contentLength, nil
}
