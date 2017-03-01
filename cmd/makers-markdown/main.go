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
	Name               string
	Prerequisites      []string
	Filename, Markdown string
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
					Name:     tgt,
					Markdown: "",
					// Assumes target name is safe for filenames
					Filename: fmt.Sprintf("%s.md", tgt),
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

func generateMarkdown(makefile *Makefile, outdir, outfile string, split, merge bool) error {
	var fMergeOut *os.File
	var err error

	header := "<!--\n";
	header += fmt.Sprintf("\tGenerated on:\t%s\n", time.Now());
	header += fmt.Sprintf("\tFrom:\t%s\n", makefile.Source);
	header += "\n\tDO NOT MANUALLY EDIT THIS FILE.\n\tYOUR CHANGES WILL BE LOST NEXT TIME IT'S GENERATED\n";
	header += "-->\n\n";

	fMergeOut, err = os.OpenFile(fmt.Sprintf("%s/%s", outdir, outfile),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer fMergeOut.Close()

	fMergeOut.WriteString(header)
	if len(*makefile.Targets) > 0 {
		fMergeOut.WriteString("\n# Targets\n")
		for _, tgt := range *makefile.Targets {
			if merge {
				fMergeOut.WriteString(fmt.Sprintf("1. <a href=\"#%s\">`%s`</a>\n", tgt.Name, tgt.Name))
			} else {
				fMergeOut.WriteString(fmt.Sprintf("1. <a href=\"%s\">`%s`</a>\n", tgt.Filename, tgt.Name))
			}
		}
		fMergeOut.WriteString("\n\n___\n\n\n")
	}

	for _, tgt := range *makefile.Targets {
		if split {
			fTgt, err := os.OpenFile(fmt.Sprintf("%s/%s", outdir, tgt.Filename),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				return err
			}
			defer fTgt.Close()
			fTgt.WriteString(header)
			processTarget(&tgt, fTgt)
		}

		if merge {
			processTarget(&tgt, fMergeOut)
		}
	}

	return nil
}

func processTarget(tgt *Target, fOut *os.File) {
	fOut.WriteString(fmt.Sprintf("### <a name=\"%s\">`%s`</a>\n", tgt.Name, tgt.Name))
	if len(tgt.Prerequisites) > 0 {
		fOut.WriteString("Pre-Requisites: ")
		for _, req := range tgt.Prerequisites {
			if len(req) > 0 {
				fOut.WriteString(fmt.Sprintf("<a href=\"#%s\">`%s`</a>\n", req, req))
			}
		}
	}
	fOut.WriteString(tgt.Markdown)
	fOut.WriteString("\n---\n")
}

func processMakefile(infile, outdir, outfile  string, split, merge bool) error {
	buf, err := ioutil.ReadFile(infile)
	if err != nil {
		return err
	}

	fmt.Printf("Processing %s and outputting to %s\n", infile, outdir)

	content := string(buf)
	lines := strings.Split(content, "\n")
	makefile := Makefile{
		Source: infile,
		Targets: extractTargets(lines),
	}

	if len(*makefile.Targets) > 0 {
		if err = generateMarkdown(&makefile, outdir, outfile, split, merge); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Makefile (%s) contains no documented targets", infile)
	}

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
	split := c.Bool("split")
	merge := c.BoolT("merge")

	if !split && !merge {
		return fmt.Errorf("You must enable either split or merge, otherwise there will be no output!")
	}

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

	if err := processMakefile(makefile, outdir, "README.md", split, merge); err != nil {
		return err
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "Makers Markdown"
	app.HelpName = "makers-markdown"
	app.Description = "Generate markdown documentation from comments in a Makefile"
	app.Action = parseArgs

	app.Flags = []cli.Flag {
		cli.StringFlag{
			Name: "makefile",
			Value: "Makefile",
			Usage: "The target makefile to process",
		},
		cli.StringFlag{
			Name: "outdir",
			Value: "./makedocs",
			Usage: "The directory in which to write output",
		},
		cli.BoolFlag{
			Name: "split",
			Usage: "Indicates whether or not to split each target into separate files",
		},
		cli.BoolTFlag{
			Name: "merge",
			Usage: "Indicates whether or not to merge all the targets files into 1 resultant output file",
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Errorf(err.Error())
		os.Exit(1)
	}
}