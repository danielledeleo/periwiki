package pandoc

import (
	"io/ioutil"
	"log"
	"os/exec"
)

func MarkdownToHTML(input string) ([]byte, error) {
	cmd := exec.Command("pandoc", "-f", "markdown+fenced_code_attributes", "-t", "html")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()

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
