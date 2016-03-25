package workflow

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/mkrautz/plist"
	"gogs.deanishe.net/deanishe/awgo/util"
)

const (
	Version = 0.1
)

// The workflow object operated on by top-level functions.
var defaultWorkflow *Workflow

// Info contains some of the information extracted from info.plist.
type Info struct {
	BundleId    string `plist:"bundleid"`
	Author      string `plist:"createdby"`
	Description string `plist:"description"`
	Name        string `plist:"name"`
	Readme      string `plist:"readme"`
	Website     string `plist:"webaddress"`
}

// Workflow provides a simple, consolidated API for building Script
// Filters and talking to Alfred.
type Workflow struct {
	// The response that will be sent to Alfred. Workflow provides
	// convenience wrapper methods, so you don't have to interact
	// with this directly.
	Feedback Feedback

	// Alfred-specific environmental variables, without the 'alfred_'
	// prefix. The following variables are present:
	//
	//	   version                 Alfred version number, e.g. "2.7"
	//     version_build           Alfred build, e.g. "277"
	//     theme                   ID of current theme, e.g.
	//                             "alfred.theme.custom.UUID-UUID-UUID"
	//     theme_background        Theme background colour in rgba format,
	//                             e.g. "rgba(255,255,255,1.00)"
	//     theme_subtext           User's subtext setting.
	//                                 "0" = Always show
	//                                 "1" = Show only for alternate actions
	//                                 "2" = Never show
	//     preferences             Path to "Alfred.alfredpreferences" file
	//     preferences_localhash   Machine-specific hash. Machine preferences
	//                             are stored in
	//                             Alfred.alfredpreferences/preferences/local/<hash>
	//     workflow_cache          Path to workflow's cache directory. Use
	//                             Workflow.GetCacheDir() instead to ensure
	//                             directory exists.
	//     workflow_data           Path to workflow's data directory. Use
	//                             Workflow.GetDataDir() instead to ensure
	//                             directory exists.
	//     workflow_name           Name of workflow, e.g. "Fast Translator"
	//     workflow_uid            Random UID assigned to workflow by Alfred
	//     workflow_bundleid       Workflow's bundle ID from info.plist
	Env map[string]string

	// Set this to your workflow's version (used in logging)
	Version string

	info       Info
	infoLoaded bool

	bundleId    string
	name        string
	cacheDir    string
	dataDir     string
	workflowDir string
}

// readInfoPlist loads the data in `info.plist`
func (wf *Workflow) readInfoPlist() error {
	if wf.infoLoaded {
		return nil
	}

	p := path.Join(wf.GetWorkflowDir(), "info.plist")
	buf, err := ioutil.ReadFile(p)
	if err != nil {
		return fmt.Errorf("Couldn't open `info.plist` (%s) :  %v", p, err)
	}

	err = plist.Unmarshal(buf, &wf.info)
	if err != nil {
		return fmt.Errorf("Error parsing `info.plist` (%s) : %v", p, err)
	}

	wf.bundleId = wf.info.BundleId
	wf.name = wf.info.Name
	wf.infoLoaded = true
	return nil
}

// loadEnv reads Alfred's variables from the environment.
func (wf *Workflow) loadEnv() {
	wf.Env = make(map[string]string)
	// Variables currently exported by Alfred. These actual names
	// are prefixed with `alfred_`.
	keys := []string{
		"version",
		"version_build",
		"theme",
		"theme_background",
		"theme_subtext",
		"preferences",
		"preferences_localhash",
		"workflow_cache",
		"workflow_data",
		"workflow_name",
		"workflow_uid",
		"workflow_bundleid",
	}

	var val, envkey string

	for _, key := range keys {
		envkey = fmt.Sprintf("alfred_%s", key)
		val = os.Getenv(envkey)
		wf.Env[key] = val

		// Some special keys
		if key == "workflow_cache" {
			wf.cacheDir = val
		} else if key == "workflow_data" {
			wf.dataDir = val
		} else if key == "workflow_bundleid" {
			wf.bundleId = val
		} else if key == "workflow_name" {
			wf.name = val
		}
	}
}

// initializeLogging ensures future log messages are written to
// workflow's log file.
func (wf *Workflow) initializeLogging() {
	// TODO: Rotate log file
	file, err := os.OpenFile(wf.GetLogFile(),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		wf.SendErrorMsg(fmt.Sprintf("Couldn't open log file %s : %v",
			wf.GetLogFile(), err))
	}

	multi := io.MultiWriter(file, os.Stderr)
	log.SetOutput(multi)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	// log.New(multi, "", log.Ldate|log.Ltime|log.Lshortfile)
}

// GetInfo returns the metadata read from the workflow's info.plist.
func (wf *Workflow) GetInfo() Info {
	if err := wf.readInfoPlist(); err != nil {
		wf.SendError(err)
	}
	return wf.info
}

// GetBundleID returns the workflow's bundle ID. This library will not
// work without a bundle ID, which is set in info.plist.
func (wf *Workflow) GetBundleId() string {
	if wf.bundleId == "" { // Really old version of Alfred with no envvars?
		if err := wf.readInfoPlist(); err != nil {
			wf.SendError(err)
		}
		if wf.bundleId == "" {
			wf.SendErrorMsg("No bundle ID set in info.plist. You *must* set a bundle ID to use awgo.")
		}
	}
	return wf.bundleId
}

// GetName returns the workflow's name as specified in info.plist.
func (wf *Workflow) GetName() string {
	if wf.name == "" { // Really old version of Alfred with no envvars?
		if err := wf.readInfoPlist(); err != nil {
			wf.SendError(err)
		}
	}
	return wf.name
}

// GetWorkflowDir returns the path to the workflow's root directory.
func (wf *Workflow) GetWorkflowDir() string {
	if wf.workflowDir == "" {
		dir, err := util.GetWorkflowRoot()
		if err != nil {
			wf.SendError(err)
		}
		wf.workflowDir = dir
	}
	return wf.workflowDir
}

// GetCacheDir returns the path to the workflow's cache directory.
// The directory will be created if it does not already exist.
func (wf *Workflow) GetCacheDir() string {
	if wf.cacheDir == "" { // Really old version of Alfred with no envvars?
		wf.cacheDir = os.ExpandEnv(fmt.Sprintf(
			"$HOME/Library/Caches/com.runningwithcrayons.Alfred-2/Workflow Data/%s",
			wf.GetBundleId()))
	}
	return util.EnsureExists(wf.cacheDir)
}

// GetDataDir returns the path to the workflow's data directory.
// The directory will be created if it does not already exist.
func (wf *Workflow) GetDataDir() string {
	if wf.dataDir == "" { // Really old version of Alfred with no envvars?
		wf.dataDir = os.ExpandEnv(fmt.Sprintf(
			"$HOME/Library/Application Support/Alfred 2/Workflow Data/%s",
			wf.GetBundleId()))
	}
	return util.EnsureExists(wf.dataDir)
}

// GetLogFile returns the path to the workflow's log file.
func (wf *Workflow) GetLogFile() string {
	return path.Join(wf.GetCacheDir(), fmt.Sprintf("%s.log", wf.GetBundleId()))
}

// NewItem adds and returns a new feedback Item.
// See Feedback.NewItem() for more information.
func (wf *Workflow) NewItem() *Item {
	return wf.Feedback.NewItem()
}

// NewFileItem adds and returns a new feedback Item pre-populated from path.
// See Feedback.NewFileItem() for more information.
func (wf *Workflow) NewFileItem(path string) *Item {
	return wf.Feedback.NewFileItem(path)
}

// Run runs your workflow function, catching any errors.
func (wf *Workflow) Run(fn func()) {
	startTime := time.Now()
	log.Printf("-------- %s/%v (awgo/%v) --------", wf.GetName(),
		wf.Version, Version)
	// log.Println("Workflow started -------------------------")
	// log.Printf("awgo version %v", Version)

	// Catch any `panic` and display an error in Alfred.
	// SendError(Msg) will terminate the process (via log.Fatal).
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered : %x", r)
			err, ok := r.(error)
			if ok {
				wf.SendError(err)
			}
			wf.SendErrorMsg(fmt.Sprintf("%v", err))
		}
	}()

	// Call the workflow's main function.
	fn()

	elapsed := time.Now().Sub(startTime)
	log.Printf("------- %v --------", elapsed)
}

// SendError displays an error message in Alfred and quits.
func (wf *Workflow) SendError(err error) {
	msg := fmt.Sprintf("%v", err)
	wf.SendErrorMsg(msg)
}

// SendErrorMsg displays an error message in Alfred and quits.
func (wf *Workflow) SendErrorMsg(errMsg string) {
	var f Feedback
	it := f.NewItem()
	it.Title = errMsg
	it.Icon = ICON_ERROR
	if err := f.Send(); err != nil {
		log.Fatalf("Error generating XML : %v", err)
	}
	log.Fatal(errMsg)
}

// SendFeedback generates and sends the XML response to Alfred.
func (wf *Workflow) SendFeedback() {
	if err := wf.Feedback.Send(); err != nil {
		log.Fatalf("Error generating XML : %v", err)
	}
}

// NewWorkflow creates and initialises a new Workflow.
func NewWorkflow() *Workflow {
	var w Workflow
	w.loadEnv()
	w.initializeLogging()
	return &w
}

func init() {
	defaultWorkflow = NewWorkflow()
}

// GetDefaultWorkflow returns the Workflow object used by the
// package-level functions.
func GetDefaultWorkflow() *Workflow {
	return defaultWorkflow
}

// SetDefaultWorkflow changes the Workflow object used by the
// package-level functions.
func SetDefaultWorkflow(wf *Workflow) {
	defaultWorkflow = wf
}

// GetBundleId returns the bundle ID of the workflow.
// It is retrieved from Alfred's environmental variables or `info.plist`.
func GetBundleId() string {
	return defaultWorkflow.GetBundleId()
}

// GetName returns the name of the workflow.
func GetName() string {
	return defaultWorkflow.GetName()
}

// GetCacheDir returns the path to the workflow's cache directory.
// The directory will be created if it does not already exist.
func GetCacheDir() string {
	return defaultWorkflow.GetCacheDir()
}

// GetDataDir returns the path to the workflow's data directory.
// The directory will be created if it does not already exist.
func GetDataDir() string {
	return defaultWorkflow.GetDataDir()
}

// GetWorkflowDir returns the path to the workflow's root directory.
func GetWorkflowDir() string {
	return defaultWorkflow.GetWorkflowDir()
}

// NewItem adds and returns a new feedback Item.
// See Feedback.NewItem() for more information.
func NewItem() *Item {
	return defaultWorkflow.NewItem()
}

// NewFileItem adds and returns an Item pre-populated from path.
// See Feedback.NewFileItem() for more information.
func NewFileItem(path string) *Item {
	return defaultWorkflow.NewFileItem(path)
}

// SendError sends an error message to Alfred as XML feedback and
// terminates the workflow.
func SendError(err error) {
	defaultWorkflow.SendError(err)
}

// SendErrorMsg sends an error message to Alfred as XML feedback and
// terminates the workflow.
func SendErrorMsg(errMsg string) {
	defaultWorkflow.SendErrorMsg(errMsg)
}

// SendFeedback generates and sends the XML response to Alfred.
// The XML is output to STDOUT. At this point, Alfred considers your
// workflow complete; sending further responses will have no effect.
func SendFeedback() {
	defaultWorkflow.SendFeedback()
}

// Run runs your workflow function, catching any errors.
func Run(fn func()) {
	defaultWorkflow.Run(fn)
}