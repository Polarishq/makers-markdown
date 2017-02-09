package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
	"io/ioutil"
	"strings"
	"regexp"
	"time"
)

type Makefile struct {
	Source string
	Targets []Target
}

type Target struct {
	Name string
	Prerequisites []string
	Markdown string
}

func getMatches(myExp *regexp.Regexp, s string) map[string]string {
	match  := myExp.FindStringSubmatch(s)
	result := make(map[string]string)
	for i, name := range myExp.SubexpNames() {
		if i != 0 && i < len(match) {
			v := match[i]
			result[name] = v
		}
	}
	return result
}

func extractTargets(lines []string) []Target {
	// Regular expression to match Makefile targets
	reTgt := regexp.MustCompile(`(?P<tgt>^[a-zA-Z0-9_-]+):(?P<prereqs>.*)`)	// Naive match, works for now
	reComment := regexp.MustCompile(`^\s*#{1}(?P<markdown>.*)$`)

	scanning := true
	targets := make([]Target, 0)

	for _, s := range lines {
		var target Target

		if scanning {
			matches := getMatches(reTgt, s)
			if tgt, ok := matches["tgt"]; ok {
				target = Target {
					Name: tgt,
				}

				// Highlight the prerequisite targets
				if prereqs, ok := matches["prereqs"]; ok {
					tgts := strings.Split(prereqs, " ")
					if len(tgts) > 1 {
						prereqs := make([]string, 0, 1)
						for _, req := range tgts {
							if len(req) > 0 {
								prereqs = append(prereqs, req)
							}
						}
					}
				}

				// switch to output mode
				scanning = false
			}
		} else {
			matches := getMatches(reComment, s)
			markdown, ok := matches["markdown"]
			if ok {
				target.Markdown += fmt.Sprintf("%s\n", markdown)
				continue
			}

			// Dropping through to here means we hit the end of the comments
			scanning = true
		}
	}
	return targets
}

func generateMarkdown(makefile Makefile, fOut *os.File) {

	fOut.WriteString(fmt.Sprintf("\n\n<!-- Generated on %s from %s -->\n", time.Now(), makefile))

	for _, tgt := range makefile.Targets {
		// Found a Makefile makefile, Output a header
		fOut.WriteString(fmt.Sprintf("\n\n<a name=\"%s\">&nbsp;</a>\n", tgt ))
		fOut.WriteString(fmt.Sprintf("# Target `%s`\n", tgt))
		if len(tgt.Prerequisites) > 0 {
			fOut.WriteString("Pre-Requisites: ")
			for _, req := range tgt.Prerequisites {
				if len(req) > 0 {
					fOut.WriteString(fmt.Sprintf("<a href=\"#%s\">%s</a>\n", req, req))
				}
			}
		}
	}
}

func processMakefile(makefile, outdir, outfile  string) error {
	buf, err := ioutil.ReadFile(makefile)
	if err != nil {
		return err
	}

	content := string(buf)
	lines := strings.Split(content, "\n")
	mk := Makefile{
		Source: makefile,
		Targets: extractTargets(lines),
	}

	fOut, err := os.OpenFile(fmt.Sprintf("%s/%s", outdir, outfile),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer fOut.Close()

	generateMarkdown(mk, fOut)
	return nil
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
	return true, err
}

func parseArgs(c *cli.Context) error {
	target := c.String("target")
	outdir := c.String("outdir")

	fmt.Printf("Processing %s and outputting to %s\n", target, outdir)

	dirExists, err := exists(outdir)
	if err != nil {
		return err
	}
	if !dirExists {
		if err := os.MkdirAll(outdir,0777); err != nil {
			return err
		}
	}

	if err := processMakefile(target, outdir, "README.md"); err != nil {
		return err
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Action = parseArgs

	app.Flags = []cli.Flag {
		cli.StringFlag{
			Name: "makefile",
			Value: "Makefile",
			Usage: "The target makefile to process",
		},
		cli.StringFlag{
			Name: "outdir",
			Value: "./docs",
			Usage: "The directory in which to write output",
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Errorf(err.Error())
		os.Exit(1)
	}
}