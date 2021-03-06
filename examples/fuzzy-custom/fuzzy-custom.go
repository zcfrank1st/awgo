//
// Copyright (c) 2017 Dean Jackson <deanishe@deanishe.net>
//
// MIT Licence. See http://opensource.org/licenses/MIT
//
// Created on 2017-09-09
//

/*
Workflow fuzzy-custom is a complete Alfred 3 workflow.

It demonstrates how to implement fuzzy.Interface on your own structs, based
on a list of Alfred workflows from GitHub.

The list of workflows is generated by the alfred_workflows.py script,
which queries the GitHub API for repos matching the topic "alfred-workflow".
*/
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"git.deanishe.net/deanishe/awgo"
	"git.deanishe.net/deanishe/awgo/fuzzy"
	"git.deanishe.net/deanishe/awgo/util"
	"github.com/docopt/docopt-go"
)

var (
	maxResults   = 200
	workflowJSON = "workflows.json"
	usage        = `fuzzy-custom [options] [<query>]

Usage:
    fuzzy-custom <query>
    fuzzy-custom -h|--version

Options:
    -h, --help  Show this message and exit
    --version   Show version number and exit
`
	sopts []fuzzy.Option
	wf    *aw.Workflow
)

func init() {
	sopts = []fuzzy.Option{
		fuzzy.AdjacencyBonus(5.0), // default
		fuzzy.CamelBonus(10.0),    // default
		fuzzy.SeparatorBonus(15.0),
		fuzzy.LeadingLetterPenalty(-3.0), // default
		fuzzy.MaxLeadingLetterPenalty(-6.0),
		fuzzy.UnmatchedLetterPenalty(-1.0), // default
	}
	wf = aw.New(aw.HelpURL("http://www.deanishe.net/"))
}

// Workflow is an Alfred workflow repo from the GitHub API.
type Workflow struct {
	Name        string   `json:"name"`        // Name of the repo (w/o owner name)
	Owner       string   `json:"owner"`       // Name of repo owner
	Description string   `json:"description"` // Description of repo
	Stars       int      `json:"stars"`       // No. of stars
	Topics      []string `json:"topics"`      // GitHub topics (tags)
	URL         string   `json:"url"`         // URL of repo on GitHub
}

// Repo is the full name of the repo (owner/name).
func (w Workflow) Repo() string {
	return fmt.Sprintf("%s/%s", w.Owner, w.Name)
}

// Workflows is a slice of Workflows that supports fuzzy.Interface.
type Workflows []*Workflow

// Implement sort.Interface
func (s Workflows) Len() int           { return len(s) }
func (s Workflows) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s Workflows) Less(i, j int) bool { return s[i].Repo() < s[j].Repo() }

// SortKey implements fuzzy.Interface.
func (s Workflows) SortKey(i int) string {
	// return s[i].Name
	return fmt.Sprintf("%s %s", s[i].Owner, s[i].Name)
}

// Filter fuzzy-sorts workflows and returns a copy without non-matching ones.
func (s Workflows) Filter(query string, max int) Workflows {
	start := time.Now()
	sorter := fuzzy.New(s, sopts...)

	res := sorter.Sort(query)

	var n int
	hits := s[:0]
	for i, w := range s {
		r := res[i]
		// Ignore workflows that don't match (i.e. not all characters in query
		// are in the workflow)
		if r.Match {
			n++
			hits = append(hits, w)
			log.Printf("[fuzzy] %3d. score=%5.2f, owner=%s, name=%s", n, r.Score, w.Owner, w.Name)
			if max > 0 && n == max {
				break
			}
		}
	}

	log.Printf("[fuzzy] sorted %d workflows in %v", s.Len(), util.ReadableDuration(time.Now().Sub(start)))
	return hits
}

// loadWorkflows loads the list of workflows from the workflow's directory.
func loadWorkflows() (Workflows, error) {
	start := time.Now()
	path := filepath.Join(wf.Dir(), workflowJSON)
	// Unmarshal workflows.json
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	workflows := Workflows{}
	if err := json.Unmarshal(data, &workflows); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal JSON (%s): %v", path, err)
	}
	log.Printf("[cache] loaded %d workflows in %v", len(workflows), util.ReadableDuration(time.Now().Sub(start)))
	return workflows, nil
}

func run() {
	var query string
	args, err := docopt.Parse(usage, wf.Args(), true, wf.Version(), false)
	if err != nil {
		wf.Fatal(fmt.Sprintf("couldn't parse CLI flags: %v", err))
	}

	if s, ok := args["<query>"].(string); ok {
		query = s
	}
	log.Printf("[main] query=%s", query)

	// Load and filter podcasts
	workflows, err := loadWorkflows()
	if err != nil {
		panic(err.Error())
	}

	if query == "" {
		var n int
		for _, w := range workflows {

			wf.NewItem(w.Repo()).
				Subtitle(w.Description).
				Arg(w.URL).
				UID(w.Repo()).
				Valid(true)

			n++
			if maxResults > 0 && n == maxResults {
				break
			}
		}

		wf.WarnEmpty("No workflows", "")

		wf.SendFeedback()

		log.Printf("[main] sent %d workflows to Alfred", n)
		return
	}

	hits := workflows.Filter(query, maxResults)

	// Send results to Alfred
	for _, w := range hits {
		wf.NewItem(w.Repo()).
			Subtitle(w.Description).
			Arg(w.URL).
			UID(w.Repo()).
			Valid(true)
	}

	log.Printf("[main] %d/%d workflows match \"%s\"", hits.Len(), workflows.Len(), query)

	wf.WarnEmpty("No matching workflows", "Try a different query?")
	wf.SendFeedback()
}

func main() {
	wf.Run(run)
}
