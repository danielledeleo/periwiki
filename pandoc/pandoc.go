package pandoc

import (
	"io/ioutil"
	"log"
	"os/exec"
)

// MarkdownToHTML calls the local pandoc executable and converts the provided
// markdown to HTML.
func MarkdownToHTML(input string) ([]byte, error) {
	cmd := exec.Command("pandoc", "-f",
		"markdown+fenced_code_attributes+fenced_divs", "-t", "html")
	stdin, err := cmd.StdinPipe()

	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	defer stdout.Close()

	if err != nil {
		return nil, err
	}

	stdin.Write([]byte(input))
	stdin.Close()

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		return output, err
	}
	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}

	return output, nil
}
