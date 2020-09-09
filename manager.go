package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hashicorp/go-multierror"
	"github.com/khorevaa/r2gitsync/flow"
	"github.com/v8platform/designer"
	"github.com/v8platform/designer/repository"
	"github.com/v8platform/v8"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func syncInfobase(connString, user, password string) v8.Infobase {

	if len(connString) == 0 {
		return v8.NewTempIB()
	}
	// TODO Сделать получение базы для выполнения синхронизации
	return v8.NewTempIB()

}

type repositoryVersion struct {
	version string
	author  string
	date    time.Time
	comment string
	number  int64
}

func (r repositoryVersion) Version() string {
	return r.version
}

func (r repositoryVersion) Author() string {
	return r.author
}

func (r repositoryVersion) Date() time.Time {
	return r.date
}

func (r repositoryVersion) Comment() string {
	return r.comment
}

func (r repositoryVersion) Number() int64 {
	return r.number
}

type repositoryAuthor struct {
	name  string
	email string
}

type RepositoryAuthor interface {
	Name() string
	Email() string
	Desc() string
}

type RepositoryVersion interface {
	Version() string
	Author() string
	Date() time.Time
	Comment() string
	Number() int64
}

func (a repositoryAuthor) Name() string {
	return a.name
}

func (a repositoryAuthor) Email() string {
	return a.email
}

func (a repositoryAuthor) Desc() string {

	return fmt.Sprintf("%s <%s> ", a.name, a.email)
}

type SyncRepository struct {
	repository.Repository
	Name     string
	WorkDir  string
	Versions []repositoryVersion
	Authors  map[string]repositoryAuthor

	Extention string

	CurrentVersion   int64 `xml:"VERSION"`
	MinVersion       int64
	MaxVersion       int64
	LimitSyncVersion int64
	endpoint         V8Endpoint
	flow             flow.Flow
}

type V8Endpoint interface {
	Infobase() *v8.Infobase
	Repository() *repository.Repository
	Extention() string
	Options() []interface{}
}

func NewSyncRepository(path string) *SyncRepository {

	return &SyncRepository{
		Repository: repository.Repository{
			Path: path,
		},
	}
}

func (r *SyncRepository) Auth(user, passowrd string) {

	r.User = user
	r.Password = passowrd

}

func (r *SyncRepository) readCurrentVersion() error {

	fileVersion := path.Join(r.WorkDir, VERSION_FILE)

	// Open our xmlFile
	xmlFile, err := os.Open(fileVersion)
	// if we os.Open returns an error then handle it
	if err != nil {
		return err
	}

	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(xmlFile)

	// xmlFiles content into 'users' which we defined above
	err = xml.Unmarshal(byteValue, &r.CurrentVersion)

	if err != nil {
		return err
	}

	return nil

}

func (r *SyncRepository) sync(opts *SyncOptions) error {

	v8Endpoint := &v8Endpoint{
		infobase:   &opts.infobase,
		repository: &r.Repository,
		options:    opts.Options(),
		extention:  r.Extention,
	}

	r.endpoint = v8Endpoint

	r.flow = flow.Tasker()

	if opts.plugins.SubscribeManager != nil {

		r.flow = flow.WithSubscribes(opts.plugins.SubscribeManager)

	}

	taskFlow := r.flow

	taskFlow.StartSyncProcess(r.endpoint, r.WorkDir)
	defer taskFlow.FinishSyncProcess(r.endpoint, r.WorkDir)

	err := r.prepare(opts)

	if err != nil {
		return err
	}

	if len(r.Versions) == 0 {
		fmt.Printf("No versions to sync")
		return nil
	}

	nextVersion := r.Versions[0].number
	maxVersion := r.MaxVersion

	taskFlow.StartSyncVersions(r.endpoint, r.Versions, r.CurrentVersion, &nextVersion, &maxVersion)

	for _, rVersion := range r.Versions {

		if r.MaxVersion != 0 && rVersion.number > r.MaxVersion {
			break
		}

		err = r.syncVersionFiles(rVersion, opts)

		if err != nil {
			return err
		}

	}

	return nil
}

func (r *SyncRepository) WriteVersionFile(CurrentVersion int64) error {

	data := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<VERSION>%d</VERSION>`, CurrentVersion)

	filename := filepath.Join(r.WorkDir, VERSION_FILE)
	err := ioutil.WriteFile(filename, []byte(data), 0644)

	return err

}

func (r *SyncRepository) CommitVersionFile(author string, when time.Time, comment string) error {

	g, err := git.PlainOpen(r.WorkDir)

	if err != nil {
		return err
	}

	filename := filepath.Join(r.WorkDir, VERSION_FILE)

	w, err := g.Worktree()

	if err != nil {
		return err
	}

	w.Add(filename)

	c, err := w.Commit(comment, &git.CommitOptions{
		Author: &object.Signature{
			Name:  author,
			Email: author,
			When:  when,
		},
	})

	if err != nil {
		return err
	}

	_, err = g.CommitObject(c)

	return err
}

func (r *SyncRepository) commitFiles(author string, when time.Time, comment string) error {

	g, err := git.PlainOpen(r.WorkDir)

	if err != nil {
		return err
	}

	w, err := g.Worktree()

	if err != nil {
		return err
	}

	_ = w.AddGlob(r.WorkDir)

	c, err := w.Commit(comment, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  author,
			Email: author,
			When:  when,
		},
	})

	if err != nil {
		return err
	}

	_, err = g.CommitObject(c)

	return err

}

func (r *SyncRepository) prepare(opts *SyncOptions) error {

	if !opts.infobaseCreated {

		CreateFileInfobase := v8.CreateFileInfobase(opts.infobase.Path())

		err := Run(opts.infobase, CreateFileInfobase, opts)

		if err != nil {
			return err
		}

		opts.infobaseCreated = true
	}

	r.CurrentVersion = 0

	err := r.readCurrentVersion()

	if err != nil {
		return err
	}

	err = r.GetRepositoryAuthors(opts)

	if err != nil {
		return err
	}

	r.MaxVersion = 0

	err = r.GetRepositoryHistory(opts)
	if err != nil {
		return err
	}

	return nil
}

func (r *SyncRepository) Sync(opts ...SyncOption) error {

	options := &SyncOptions{}

	for _, o := range opts {
		o(options)
	}

	return r.sync(options)

}

func Sync(r SyncRepository, opts ...SyncOption) error {

	return r.Sync(opts...)

}

func (r *SyncRepository) syncVersionFiles(rVersion RepositoryVersion, opts *SyncOptions) (err error) {

	tempDir, err := ioutil.TempDir(opts.tempDir, fmt.Sprintf("v%d", rVersion.Number()))

	if err != nil {
		return err
	}

	flow := r.flow

	err = flow.StartSyncVersion(r.endpoint, r.WorkDir, tempDir, rVersion.Number())

	if err != nil {
		return err
	}

	defer func() {

		flow.FinishSyncVersion(r.endpoint, r.WorkDir, tempDir, rVersion.Number(), &err)

		_ = os.RemoveAll(tempDir)

	}()

	err = flow.UpdateCfg(r.endpoint, r.WorkDir, rVersion.Number())

	if err != nil {
		return err
	}

	err = flow.DumpConfigToFiles(r.endpoint, r.WorkDir, tempDir, rVersion.Number())

	if err != nil {
		return err
	}

	err = flow.ClearWorkDir(r.endpoint, r.WorkDir, tempDir)

	if err != nil {
		return err
	}

	err = flow.MoveToWorkDir(r.endpoint, r.WorkDir, tempDir)

	if err != nil {
		return err
	}

	err = flow.WriteVersionFile(r.endpoint, r.WorkDir, rVersion.Number())

	if err != nil {
		return err
	}

	err = flow.CommitFiles(r.endpoint, r.WorkDir, rVersion.Author(), rVersion.Date(), rVersion.Comment())

	if err != nil {

		errV := flow.WriteVersionFile(r.endpoint, r.WorkDir, rVersion.Number())
		if errV != nil {
			return multierror.Append(err, errV)
		}
		return err
	}

	r.CurrentVersion = rVersion.Number()

	return

}

type UpdateCfgHandlerType func(v8end V8Endpoint, workdir string, version int64) error

func (r *SyncRepository) DumpConfigToFiles(dumpDir string, opts *SyncOptions) (err error) {

	standartHandler := true

	err = opts.plugins.WithDumpCfgToFilesHandler(opts.infobase, r.Repository, r.Extention, &dumpDir, &standartHandler)

	if err != nil {
		return
	}

	if standartHandler {
		err = r.dumpConfigToFilesHandler(dumpDir, opts)
	}

	err = opts.plugins.AfterDumpCfgToFilesHandler(opts.infobase, r.Repository, r.Extention, &dumpDir)

	return

}

func (r *SyncRepository) dumpConfigToFilesHandler(dumpDir string, opts *SyncOptions) error {

	DumpConfigToFilesOptions := designer.DumpConfigToFilesOptions{
		Dir:       dumpDir,
		Force:     true,
		Extension: r.Extention,
	}

	err := Run(opts.infobase, DumpConfigToFilesOptions, opts)

	if err != nil {
		return err
	}
	return nil
}

func (r *SyncRepository) ClearWorkDir(opts *SyncOptions) (err error) {

	standartHandler := true

	err = opts.plugins.WithClearWorkdirHandler(r.WorkDir, &standartHandler)

	if err != nil {
		return
	}

	if standartHandler {
		err = r.clearWorkDirHandler(opts)
	}

	err = opts.plugins.AfterClearWorkdirHandler(r.WorkDir)

	return

}

func (r *SyncRepository) clearWorkDirHandler(opts *SyncOptions) error {

	err := os.RemoveAll(r.WorkDir) // TODO Сделать копирование файлов или избранную очистку

	if err != nil {
		return err
	}
	return nil
}

func (r *SyncRepository) MoveToWorkDir(dumpDir string, opts *SyncOptions) (err error) {

	standartHandler := true

	err = opts.plugins.WithMoveToWorkDirHandler(r.WorkDir, dumpDir, &standartHandler)

	if err != nil {
		return
	}

	if standartHandler {
		err = r.moveToWorkDirHandler(dumpDir, opts)
	}

	err = opts.plugins.AfterMoveToWorkDirHandler(r.WorkDir, dumpDir)

	return

}

func (r *SyncRepository) moveToWorkDirHandler(dumpDir string, opts *SyncOptions) error {

	err := os.RemoveAll(r.WorkDir) // TODO Сделать копирование файлов или избранную очистку

	if err != nil {
		return err
	}
	return nil
}

func (r *SyncRepository) GetRepositoryHistory(opts *SyncOptions) (err error) {

	standartHandler := true

	err = opts.plugins.WithGetRepositoryHistoryHandler(r.WorkDir, r.Repository, &r.Versions, opts, &standartHandler)

	if err != nil {
		return
	}

	if standartHandler {
		err = r.getRepositoryHistoryHandler(opts)
		if err != nil {
			return
		}

	}

	err = opts.plugins.AfterGetRepositoryHistoryHandler(r.WorkDir, r.Repository, &r.Versions)

	return

}

func (r *SyncRepository) getRepositoryHistoryHandler(opts *SyncOptions) error {

	reportFile, err := ioutil.TempFile(opts.tempDir, "v8_rep_history")
	if err != nil {
		return err
	}
	reportFile.Close()
	report := reportFile.Name()

	defer os.Remove(report)

	RepositoryReportOptions := repository.RepositoryReportOptions{
		Repository: r.Repository,
		File:       report,
		Extension:  r.Extention,
		NBegin:     r.CurrentVersion,
	}.GroupByComment()

	err = Run(opts.infobase, RepositoryReportOptions, opts)

	if err != nil {
		return err
	}

	r.Versions, err = parseRepositoryReport(report)

	if err != nil {
		return err
	}

	sort.Slice(r.Versions, func(i, j int) bool {
		return r.Versions[i].Number() < r.Versions[j].Number()
	})

	if len(r.Versions) > 0 {
		r.MaxVersion = r.Versions[len(r.Versions)-1].Number()
	}

	return nil
}

func (r *SyncRepository) GetRepositoryAuthors(opts *SyncOptions) (err error) {

	standartHandler := true
	authors := new(AuthorsList)

	err = opts.plugins.WithGetRepositoryAuthorsHandler(r.WorkDir, r.Repository, authors, opts, &standartHandler)

	if err != nil {
		r.Authors = authors
		return
	}

	if standartHandler {
		err = r.getRepositoryAuthorsHandler(authors, opts)
	}

	err = opts.plugins.AfterGetGetRepositoryAuthorsHandler(r.WorkDir, r.Repository, authors)

	return

}

func (r *SyncRepository) getRepositoryAuthorsHandler(authors *AuthorsList, opts *SyncOptions) error {

	file := path.Join(r.WorkDir, AUTHORS_FILE)
	_, err := os.Lstat(file)

	r.Authors = new(AuthorsList)
	authorsTable := *r.Authors
	if err != nil {
		authors = &authorsTable
		return nil
	}

	bytesRead, _ := ioutil.ReadFile(file)
	file_content := string(bytesRead)
	lines := strings.Split(file_content, "\n")

	for _, line := range lines {

		line = strings.TrimSpace(line)

		if len(line) == 0 || strings.HasPrefix(line, "//") {
			continue
		}

		data := strings.Split(line, "=")

		authorsTable[data[0]] = NewAuthor(decodeAuthor([]byte(data[1])))

	}

	r.Authors = &authorsTable

	return nil

}

func NewAuthor(name, email string) author {

	return author{
		Name:  name,
		Email: email,
	}

}

// Decode decodes a byte slice into a signature
func decodeAuthor(b []byte) (string, string) {
	open := bytes.LastIndexByte(b, '<')
	closeSym := bytes.LastIndexByte(b, '>')
	if open == -1 || closeSym == -1 {
		return "", ""
	}

	if closeSym < open {
		return "", ""
	}

	Name := string(bytes.Trim(b[:open], " "))
	Email := string(b[open+1 : closeSym])

	return Name, Email

}
