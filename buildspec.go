package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-steputils/output"
	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/kballard/go-shellquote"
	"github.com/ryanuber/go-glob"
)

type buildSpecification struct {
	displayName          string
	platformOutputType   OutputType
	platformSelectors    []string
	outputPathPatterns   []string
	additionalParameters string
	projectLocation      string
}

func (spec buildSpecification) exportArtifacts(artifacts []string) error {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	switch spec.platformOutputType {
	case OutputTypeAPK:
		return spec.exportAndroidArtifacts(OutputTypeAPK, artifacts, deployDir)
	case OutputTypeAppBundle:
		return spec.exportAndroidArtifacts(OutputTypeAppBundle, artifacts, deployDir)
	case OutputTypeIOSApp:
		return spec.exportIOSApp(artifacts, deployDir)
	case OutputTypeArchive:
		return spec.exportIOSArchive(artifacts, deployDir)
	default:
		return fmt.Errorf("unsupported platform for exporting artifacts: %s. Supported platforms: apk, appbundle, app, archive", spec.platformOutputType)
	}
}

func (spec buildSpecification) artifactPaths(outputPathPatterns []string, isDir bool) ([]string, error) {
	var paths []string
	for _, outputPathPattern := range outputPathPatterns {
		pths, err := findPaths(spec.projectLocation, outputPathPattern, isDir)
		if err != nil {
			return nil, err
		}
		paths = append(paths, pths...)
	}
	return paths, nil
}

func (spec buildSpecification) exportIOSApp(artifacts []string, deployDir string) error {
	artifact := artifacts[len(artifacts)-1]
	fileName := filepath.Base(artifact)

	if len(artifacts) > 1 {
		log.Warnf("- Multiple artifacts found: %v, exporting %s", artifacts, artifact)
	}

	if err := ziputil.ZipDir(artifact, filepath.Join(deployDir, fileName+".zip"), false); err != nil {
		return err
	}
	log.Donef("- $BITRISE_DEPLOY_DIR/" + fileName + ".zip")

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_APP_DIR_PATH", artifact); err != nil {
		return err
	}
	log.Donef("- $BITRISE_APP_DIR_PATH: " + artifact)

	return nil
}

func (spec buildSpecification) exportIOSArchive(artifacts []string, deployDir string) error {
	artifact := artifacts[len(artifacts)-1]
	fileName := filepath.Base(artifact)

	if len(artifacts) > 1 {
		log.Warnf("- Multiple artifacts found: %v, exporting %s", artifacts, artifact)
	}

	zipPath := filepath.Join(deployDir, fileName+".zip")
	if err := ziputil.ZipDir(artifact, zipPath, false); err != nil {
		return err
	}
	log.Donef("- $BITRISE_DEPLOY_DIR/" + fileName + ".zip")

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_XCARCHIVE_PATH", artifact); err != nil {
		return err
	}
	log.Donef("- $BITRISE_XCARCHIVE_PATH: " + artifact)

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_XCARCHIVE_ZIP_PATH", zipPath); err != nil {
		return err
	}
	log.Donef("- BITRISE_XCARCHIVE_ZIP_PATH: " + zipPath)

	return nil
}

func (spec buildSpecification) exportAndroidArtifacts(androidOutputType OutputType, artifacts []string, deployDir string) error {
	artifacts = filterAndroidArtifactsBy(androidOutputType, artifacts)

	var singleFileOutputEnvName string
	var multipleFileOutputEnvName string
	switch spec.platformOutputType {
	case "appbundle":
		singleFileOutputEnvName = "BITRISE_AAB_PATH"
		multipleFileOutputEnvName = "BITRISE_AAB_PATH_LIST"
	default:
		singleFileOutputEnvName = "BITRISE_APK_PATH"
		multipleFileOutputEnvName = "BITRISE_APK_PATH_LIST"
	}

	var deployedFiles []string
	for _, path := range artifacts {
		deployedFilePath := filepath.Join(deployDir, filepath.Base(path))

		if err := output.ExportOutputFile(path, deployedFilePath, singleFileOutputEnvName); err != nil {
			return err
		}
		deployedFiles = append(deployedFiles, deployedFilePath)
	}
	if err := tools.ExportEnvironmentWithEnvman(multipleFileOutputEnvName, strings.Join(deployedFiles, "\n")); err != nil {
		return fmt.Errorf("failed to export enviroment variable %s, error: %s", multipleFileOutputEnvName, err)
	}

	deployedSingleFile := ""
	if len(deployedFiles) > 0 {
		deployedSingleFile = deployedFiles[len(deployedFiles)-1]
	}

	log.Donef("- " + singleFileOutputEnvName + ": " + deployedSingleFile)
	log.Donef("- " + multipleFileOutputEnvName + ": " + strings.Join(deployedFiles, "|"))
	return nil
}

func filterAndroidArtifactsBy(androidOutputType OutputType, artifacts []string) []string {
	var index int
	for _, artifact := range artifacts {
		switch androidOutputType {
		case OutputTypeAPK:
			if path.Ext(artifact) != ".apk" {
				log.Debugf("Artifact (%s) found by output patterns, but it's not the selected output type (%s) - Skip", artifact, androidOutputType)
				continue // drop artifact
			}
		case OutputTypeAppBundle:
			if path.Ext(artifact) != ".aab" {
				log.Debugf("Artifact (%s) found by output patterns, but it's not the selected output type (%s) - Skip", artifact, androidOutputType)
				continue // drop artifact
			}
		}
		artifacts[index] = artifact
		index++
	}
	return artifacts[:index]
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
		log.Debugf("couldn't find output artifact on path: " + filepath.Join(location, outputPathPattern))
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

	var platformCmd string
	switch spec.platformOutputType {
	case OutputTypeIOSApp:
		platformCmd = "ios" // $ flutter build ios -> .app output
	case OutputTypeArchive:
		platformCmd = "ipa" // $ flutter build ipa -> .xcarchive output
	default:
		platformCmd = string(spec.platformOutputType)
	}

	if spec.platformOutputType == OutputTypeIOSApp {
		paramSlice = append(paramSlice, "--no-codesign")
	}

	buildCmd := command.New("flutter", append([]string{"build", platformCmd}, paramSlice...)...).SetStdout(os.Stdout)

	if spec.platformOutputType == OutputTypeIOSApp || spec.platformOutputType == OutputTypeArchive {
		buildCmd.SetStdin(strings.NewReader("a")) // if the CLI asks to input the selected identity we force it to be aborted
		errorWriter = io.MultiWriter(os.Stderr, &errBuffer)
	}

	buildCmd.SetStderr(errorWriter)

	fmt.Println()
	log.Donef("$ %s", buildCmd.PrintableCommandArgs())
	fmt.Println()

	buildCmd.SetDir(spec.projectLocation)

	err = buildCmd.Run()

	if spec.platformOutputType == OutputTypeIOSApp {
		if strings.Contains(strings.ToLower(errBuffer.String()), "code signing is required") {
			return errCodeSign
		}
	}

	return err
}
