package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

type crawler struct {
	startURL     *url.URL
	destDir      string
	visitedURLs  map[string]bool
	visitedMutex sync.Mutex
}

func newCrawler(url *url.URL, destination string, visited map[string]bool) (*crawler, error) {
	return &crawler{
		startURL:     url,
		destDir:      destination,
		visitedURLs:  visited,
		visitedMutex: sync.Mutex{},
	}, nil
}

// wgetMirrorCmd represents the wgetMirror command
var wgetMirrorCmd = &cobra.Command{
	Use:   "wgetMirror",
	Short: "wget mirror command clone",
	Long:  `A command that tries to mimic the wget --mirror command`,
	RunE: func(cmd *cobra.Command, args []string) error {
		url, destination, argsErr := readAndValidateArgs(args)
		if argsErr != nil {
			return fmt.Errorf("error: %v", argsErr)
		}

		visited, loadErr := loadAlreadyVisitedFiles(destination)
		if loadErr != nil {
			return fmt.Errorf("error: could not load already visited files")
		}

		crawler, creationErr := newCrawler(url, destination, visited)
		if creationErr != nil {
			return fmt.Errorf("error: could not create crawler")
		}

		downloadErr := crawler.download(cmd.Context(), url)

		if downloadErr != nil {
			return fmt.Errorf("error: could not download file")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(wgetMirrorCmd)
}

func readAndValidateArgs(args []string) (*url.URL, string, error) {
	if len(args) < 2 {
		return nil, "", errors.New("error: not enough arguments. Please provide a start URL and a destination directory - wgetMirror [startUrl] [destinationDirectory]")
	}

	startURL := args[0]
	sUrl, parseErr := url.Parse(startURL)
	if parseErr != nil {
		return nil, "", errors.New("error: invalid start URL")
	}

	destination := args[1]
	if !createDestinationDirectory(destination) {
		return nil, "", errors.New("error: invalid destination directory")
	}

	return sUrl, destination, nil
}

func createDestinationDirectory(destination string) bool {
	folderInfo, osErr := os.Lstat(destination)
	if osErr != nil && os.IsNotExist(osErr) {
		osErr = os.MkdirAll(destination, 0755)
		if osErr != nil {
			return false
		}

		return true
	}

	return folderInfo.IsDir()
}

func loadAlreadyVisitedFiles(destination string) (map[string]bool, error) {
	visited := make(map[string]bool)

	files, readErr := os.ReadDir(destination)
	if readErr != nil {
		return nil, readErr
	}

	for _, file := range files {
		visited[strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))] = true
	}

	return visited, nil
}

func (c *crawler) download(ctx context.Context, url *url.URL) error {
	notifyCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error

	normalizedUrl := c.normalizeFilename(url.String())

	c.visitedMutex.Lock()
	visited := c.visitedURLs[normalizedUrl]
	c.visitedURLs[normalizedUrl] = true
	c.visitedMutex.Unlock()

	if visited {
		return nil
	}

	// Download the page.
	resp, getErr := http.Get(url.String())
	if getErr != nil {
		return getErr
	}
	defer resp.Body.Close()

	// Create a file in the destination directory with the same name as the URL path.
	filename := normalizedUrl
	if filename == "" {
		filename = "index.html"
	}
	destPath := path.Join(c.destDir, filename)
	_, statErr := os.Stat(destPath)
	if statErr == nil {
		// File already exists, skip processing links.
		return nil
	}

	// Write the response body to the destination file.
	var htmlBody bytes.Buffer
	body := io.TeeReader(resp.Body, &htmlBody)
	destFile, osErr := os.Create(destPath)
	if osErr != nil {
		return osErr
	}
	defer destFile.Close()

	_, ioErr := io.Copy(destFile, body)
	if ioErr != nil {
		return ioErr
	}

	doc, readerErr := goquery.NewDocumentFromReader(bytes.NewReader(htmlBody.Bytes()))
	if readerErr != nil {
		return fmt.Errorf("error: could not read html body: %v", readerErr)
	}

	wg := sync.WaitGroup{}
	for _, link := range doc.Find("a[href]").Nodes {
		href := link.Attr[0].Val
		linkURL, parseErr := url.Parse(href)
		if parseErr != nil {
			continue
		}
		absURL := url.ResolveReference(linkURL)
		if !c.isChildURL(absURL) || strings.Contains(absURL.String(), "#") {
			continue
		}
		wg.Add(1)
		go func() {
			wgErr := func() error {
				defer wg.Done()
				select {
				case <-notifyCtx.Done():
					fmt.Println("Exiting...")
					return nil
				default:
					return c.download(ctx, absURL)
				}
			}()
			if wgErr != nil {
				err = fmt.Errorf("error: could not download link: %v", wgErr)
			}
		}()
	}

	wg.Wait()

	return err
}

func (c *crawler) isChildURL(u *url.URL) bool {
	if u.Scheme != c.startURL.Scheme || u.Host != c.startURL.Host {
		return false
	}

	return strings.HasPrefix(u.Path, c.startURL.Path+"/") || u.Path == c.startURL.Path
}

func (c *crawler) normalizeFilename(filename string) string {
	result := strings.ReplaceAll(filename, ":", "")
	result = strings.ReplaceAll(result, ".", "_")
	return strings.ReplaceAll(result, "/", "_")
}
