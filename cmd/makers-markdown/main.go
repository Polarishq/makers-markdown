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
	Source  string
	Targets *[]Target
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

func extractTargets(lines []string) *[]Target {
	// Regular expression to match Makefile targets
	reTgt := regexp.MustCompile(`(?P<tgt>^[a-zA-Z0-9_-]+):(?P<prereqs>.*)`)	// Naive match, works for now
	reComment := regexp.MustCompile(`^\s*#{1}(?P<markdown>.*)$`)

	scanning := true
	targets := make([]Target, 0)

	var target Target

	for _, s := range lines {
		if scanning {
			matches := getMatches(reTgt, s)
			if tgt, ok := matches["tgt"]; ok {
				target = Target {
					Name: tgt,
					Markdown: "",
				}

				// Highlight the prerequisite targets
				if prereqs, ok := matches["prereqs"]; ok {
					tgts := strings.Split(prereqs, " ")
					if len(tgts) > 1 {
						//target.Prerequisites = make([]string, 0, 1)
						for _, req := range tgts {
							if len(req) > 0 {
								target.Prerequisites = append(target.Prerequisites, req)
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
			if target.Markdown != "" {
				targets = append(targets, target)
			}
			scanning = true
		}
	}
	return &targets
}

func generateMarkdown(makefile *Makefile, fOut *os.File) {

	fOut.WriteString("<!--\n")
	fOut.WriteString(fmt.Sprintf("\tGenerated on:\t%s\n", time.Now()))
	fOut.WriteString(fmt.Sprintf("\tFrom:\t%s\n", makefile.Source))
	fOut.WriteString("\n\tDO NOT MANUALLY EDIT THIS FILE.\n\tYOUR CHANGES WILL BE LOST NEXT TIME IT'S GENERATED\n")
	fOut.WriteString("-->\n\n")

	if len(*makefile.Targets) > 0 {
		fOut.WriteString("\n# Targets\n")
		for _, tgt := range *makefile.Targets {
			fOut.WriteString(fmt.Sprintf("1. <a href=\"#%s\">`%s`</a>\n", tgt.Name, tgt.Name))
		}

		fOut.WriteString("\n\n___\n\n\n")

		for _, tgt := range *makefile.Targets {
			fOut.WriteString(fmt.Sprintf("### <a name=\"%s\">`%s`</a>\n", tgt.Name, tgt.Name))
			if len(tgt.Prerequisites) > 0 {
				fOut.WriteString("Pre-Requisites: ")
				for _, req := range tgt.Prerequisites {
					if len(req) > 0 {
						fOut.WriteString(fmt.Sprintf("<a href=\"#%s\">%s</a>\n", req, req))
					}
				}
			}
			fOut.WriteString(tgt.Markdown)
			fOut.WriteString("\n---\n")
		}
	}
}

func parseMakefile(makefile, outdir, outfile  string) error {
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

	generateMarkdown(&mk, fOut)
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
	makefile := c.String("makefile")
	outdir := c.String("outdir")

	fmt.Printf("Processing %s and outputting to %s\n", makefile, outdir)

	fileExists, err := exists(makefile)
	if err != nil {
		return err
	}
	if !fileExists {
		return fmt.Errorf("Input file does not exist: '%s'", makefile)
	}

	dirExists, err := exists(outdir)
	if err != nil {
		return err
	}
	if !dirExists {
		if err := os.MkdirAll(outdir,0777); err != nil {
			return err
		}
	}

	if err := parseMakefile(makefile, outdir, "README.md"); err != nil {
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