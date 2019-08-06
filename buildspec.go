package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
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
	outputPathPatterns   []string
	additionalParameters string
	projectLocation      string
}

func (spec buildSpecification) exportArtifacts(outputPathPatterns []string, androidOutputType AndroidArtifactType) error {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	switch spec.platformCmdFlag {
	case "apk", "appbundle":
		return spec.exportAndroidArtifacts(androidOutputType, outputPathPatterns, deployDir)
	case "ios":
		var paths []string
		for _, outputPathPattern := range outputPathPatterns {
			pths, err := findPaths(spec.projectLocation, outputPathPattern, true)
			if err != nil {
				return err
			}
			paths = append(paths, pths...)
		}

		path := paths[len(paths)-1]
		if len(paths) > 1 {
			log.Warnf("- Multiple artifacts found for pattern \"%s\": %v, exporting %s", outputPathPatterns, paths, path)
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

func (spec buildSpecification) exportAndroidArtifacts(androidOutputType AndroidArtifactType, outputPathPatterns []string, deployDir string) error {
	var paths []string
	for _, outputPathPattern := range outputPathPatterns {
		pths, err := findPaths(spec.projectLocation, outputPathPattern, false)
		if err != nil {
			return err
		}
		paths = append(paths, pths...)
	}
	paths = filterAndroidArtifactsBy(androidOutputType, paths)

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
		return err
	}

	log.Donef("- " + singleFileOutputEnvName + ": " + deployedFiles[len(deployedFiles)-1])
	log.Donef("- " + multipleFileOutputEnvName + ": " + strings.Join(deployedFiles, "|"))
	return nil
}

func filterAndroidArtifactsBy(androidOutputType AndroidArtifactType, artifacts []string) []string {
	var index int
	for _, artifact := range artifacts {
		switch androidOutputType {
		case APK:
			if path.Ext(artifact) != ".apk" {
				continue // drop artifact
			}
		case AppBundle:
			if path.Ext(artifact) != ".aab" {
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
