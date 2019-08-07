package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-tools/go-steputils/output"
	"github.com/bitrise-tools/go-steputils/tools"
	"github.com/kballard/go-shellquote"
	"github.com/ryanuber/go-glob"
)

type buildSpecification struct {
	displayName          string
	platformCmdFlag      string
	platformSelectors    []string
	outputPathPattern    string
	additionalParameters string
	projectLocation      string
}

func (spec buildSpecification) exportArtifacts(outputPathPattern string) error {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	switch spec.platformCmdFlag {
	case "apk":
		fallthrough
	case "appbundle":
		return spec.exportAndroidArtifacts(outputPathPattern, deployDir)
	case "ios":
		paths, err := findPaths(spec.projectLocation, outputPathPattern, true)
		if err != nil {
			return err
		}

		path := paths[len(paths)-1]
		if len(paths) > 1 {
			log.Warnf("- Multiple artifacts found for pattern \"%s\": %v, exporting %s", outputPathPattern, paths, path)
		}

		fileName := filepath.Base(path)

		if err := ziputil.ZipDir(path, filepath.Join(deployDir, fileName+".zip"), false); err != nil {
			return err
		}
		log.Donef("- $BITRISE_DEPLOY_DIR/" + fileName + ".zip")

		if err := tools.ExportEnvironmentWithEnvman("BITRISE_APP_DIR_PATH", path); err != nil {
			return err
		}
		log.Donef("- $BITRISE_APP_DIR_PATH: " + path)

		return nil
	default:
		return fmt.Errorf("unsupported platform for exporting artifacts")
	}
}

func (spec buildSpecification) exportAndroidArtifacts(outputPathPattern string, deployDir string) error {
	paths, err := findPaths(spec.projectLocation, outputPathPattern, false)
	if err != nil {
		return err
	}

	var singleFileOutputEnvName string
	var multipleFileOutputEnvName string
	switch spec.platformCmdFlag {
	case "appbundle":
		singleFileOutputEnvName = "BITRISE_AAB_PATH"
		multipleFileOutputEnvName = "BITRISE_AAB_PATH_LIST"
	default:
		singleFileOutputEnvName = "BITRISE_APK_PATH"
		multipleFileOutputEnvName = "BITRISE_APK_PATH_LIST"
	}

	var deployedFiles []string
	for _, path := range paths {
		deployedFilePath := filepath.Join(deployDir, filepath.Base(path))

		if err := output.ExportOutputFile(path, deployedFilePath, singleFileOutputEnvName); err != nil {
			return err
		}
		deployedFiles = append(deployedFiles, deployedFilePath)
	}
	if err := tools.ExportEnvironmentWithEnvman(multipleFileOutputEnvName, strings.Join(deployedFiles, "\n")); err != nil {

	}

	log.Donef("- " + singleFileOutputEnvName + ": " + deployedFiles[len(deployedFiles)-1])
	log.Donef("- " + multipleFileOutputEnvName + ": " + strings.Join(deployedFiles, "|"))
	return nil
}

func (spec buildSpecification) buildable(platform string) bool {
	return sliceutil.IsStringInSlice(platform, spec.platformSelectors)
}

func findPaths(location string, outputPathPattern string, dir bool) (out []string, err error) {
	err = filepath.Walk(location, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if !info.IsDir() == dir || !glob.Glob(outputPathPattern, path) {
			return nil
		}

		out = append(out, path)
		return nil
	})
	if len(out) == 0 && err == nil {
		err = fmt.Errorf("couldn't find output artifact on path: " + filepath.Join(location, outputPathPattern))
	}
	return
}

func (spec buildSpecification) build(params string) error {
	paramSlice, err := shellquote.Split(params)
	if err != nil {
		return err
	}

	var errorWriter io.Writer = os.Stderr
	var errBuffer bytes.Buffer

	buildCmd := command.New("flutter", append([]string{"build", spec.platformCmdFlag}, paramSlice...)...).SetStdout(os.Stdout)

	if spec.platformCmdFlag == "ios" {
		buildCmd.SetStdin(strings.NewReader("a")) // if the CLI asks to input the selected identity we force it to be aborted
		errorWriter = io.MultiWriter(os.Stderr, &errBuffer)
	}

	buildCmd.SetStderr(errorWriter)

	fmt.Println()
	log.Donef("$ %s", buildCmd.PrintableCommandArgs())
	fmt.Println()

	buildCmd.SetDir(spec.projectLocation)

	err = buildCmd.Run()

	if spec.platformCmdFlag == "ios" {
		if strings.Contains(strings.ToLower(errBuffer.String()), "code signing is required") {
			return errCodeSign
		}
	}

	return err
}
