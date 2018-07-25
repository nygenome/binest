package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/subosito/gotenv"
	"github.com/xanzy/go-gitlab"
)

func init() {
	err := gotenv.Load()
	if err != nil {
		panic(err)
	}
}

func main() {
	versionPtr := flag.String("version", "", "semantic version to be deployed.")
	pidPtr := flag.Int("pid", -1, "project id to use for deployment.")

	flag.Parse()
	binaries := flag.Args()

	if *versionPtr == "" {
		fmt.Fprintln(os.Stderr, "deploy semantic version not set. cannot continue")
		flag.Usage()
		os.Exit(1)
	}

	if *pidPtr == -1 {
		fmt.Fprintln(os.Stderr, "project id not set. cannot continue")
		flag.Usage()
		os.Exit(1)
	}

	if len(binaries) == 0 {
		fmt.Fprintln(os.Stderr, "no binaries provided to deploy. cannot continue")
		flag.Usage()
		os.Exit(1)
	}

	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "gitlab access token not set. cannot continue")
		flag.Usage()
		os.Exit(1)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint:gas
	}
	client := &http.Client{Transport: tr}
	glabClient := gitlab.NewClient(client, token)
	err := glabClient.SetBaseURL("https://git.nygenome.org/api/v4")
	if err != nil {
		panic(err)
	}

	links := make([]string, len(binaries))
	for _, binary := range binaries {
		pfile, _, err2 := glabClient.Projects.UploadFile(*pidPtr, binary)
		if err2 != nil {
			panic(err2)
		}
		links = append(links, fmt.Sprintf("* %s", pfile.Markdown))
	}

	relNotes := strings.Join(links, "\n")
	relOpts := &gitlab.CreateReleaseOptions{Description: &relNotes}
	_, _, err = glabClient.Tags.CreateRelease(*pidPtr, *versionPtr, relOpts)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "deployed snifty version %s to gitlab\n", *versionPtr)
}
