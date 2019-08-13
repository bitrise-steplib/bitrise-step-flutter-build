package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-steputils/cache"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	androidCache "github.com/bitrise-steplib/bitrise-step-android-unit-test/cache"
)

func cacheCocoapodsDeps(projectLocation string) error {
	iosDir, err := pathutil.AbsPath(filepath.Join(projectLocation, "ios"))
	if err != nil {
		return err
	}

	podfileLockPth := filepath.Join(iosDir, "Podfile.lock")
	if exist, err := pathutil.IsPathExists(podfileLockPth); err != nil {
		return err
	} else if !exist {
		return nil
	}

	podsCache := cache.New()
	podsCache.IncludePath(fmt.Sprintf("%s -> %s", filepath.Join(iosDir, "Pods"), podfileLockPth))
	return podsCache.Commit()
}

func cacheCarthageDeps(projectDir string) error {
	iosDir, err := pathutil.AbsPath(filepath.Join(projectDir, "ios"))
	if err != nil {
		return err
	}

	cartfileResolvedPth := filepath.Join(iosDir, "Cartfile.resolved")
	if exist, err := pathutil.IsPathExists(cartfileResolvedPth); err != nil {
		return err
	} else if !exist {
		return nil
	}

	carthageCache := cache.New()
	carthageCache.IncludePath(fmt.Sprintf("%s -> %s", filepath.Join(iosDir, "Carthage"), cartfileResolvedPth))
	return carthageCache.Commit()
}

func cacheAndroidDeps(projectDir string) error {
	androidDir := filepath.Join(projectDir, "android")

	exist, err := pathutil.IsDirExists(androidDir)
	if err != nil {
		return fmt.Errorf("failed to check if directory (%s) exists, error: %s", androidDir, err)
	}
	if !exist {
		return nil
	}

	return androidCache.Collect(androidDir, androidCache.LevelDeps)
}

func openPackageResolutionFile(projectDir string) (string, error) {
	resolutionFilePath := filepath.Join(projectDir, ".packages")

	if _, err := os.Stat(resolutionFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("package resolution file (%s) not found, error: %s", resolutionFilePath, err)
	}

	contents, err := ioutil.ReadFile(resolutionFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read package resolution file, error: %s", err)
	}

	return string(contents), nil
}

// parsePackageResolutionFile parses flutter package resolution file: ".package"
// https://dart.dev/tools/pub/cmd/pub-get
func parsePackageResolutionFile(contents string) (map[string]url.URL, error) {
	// Both line seperators are supported, empty lines will be ignored
	// https://github.com/lrhn/dep-pkgspec/blob/master/DEP-pkgspec.md#proposal
	contents = strings.Replace(contents, "\r", "\n", -1)
	lines := strings.Split(contents, "\n")

	packageToLocation := map[string]url.URL{}

	for _, line := range lines {
		// Empty lines are ignored (so CR+NL can be used as line separator).
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		// Lines starting with a # character (U+0023) are comments, and are otherwise ignored.
		if strings.HasPrefix(line, "#") {
			continue
		}

		// example line:
		// analyzer:file:///Users/vagrant/.pub-cache/hosted/pub.dartlang.org/analyzer-0.36.4/lib/
		packageAndLocation := strings.SplitN(line, ":", 2)
		if len(packageAndLocation) != 2 {
			return map[string]url.URL{}, fmt.Errorf("unexpected line format: %s", packageAndLocation)
		}

		location, err := url.Parse(packageAndLocation[1])
		if err != nil {
			return map[string]url.URL{}, fmt.Errorf("could not parse location URI: %s", packageAndLocation[1])
		}

		packageToLocation[packageAndLocation[0]] = *location
	}

	return packageToLocation, nil
}

func cacheableFlutterDepPaths(packageToLocation map[string]url.URL) ([]string, error) {
	var cachePaths []string

	for packageName, location := range packageToLocation {
		if location.Scheme != "file" && location.Scheme != "" {
			log.Debugf("Flutter dependency cache: ignoring non-file scheme package: %s", location.Path)
			continue
		}

		// Only care about absolute paths
		if !filepath.IsAbs(location.Path) {
			log.Debugf("Flutter dependency cache:: ignoring relative package: %s", location.Path)
			continue
		}

		sep := string(os.PathSeparator)
		location.Path = strings.TrimSuffix(location.Path, sep)
		pathElements := strings.Split(location.Path, sep)

		if len(pathElements) == 0 {
			return []string{}, fmt.Errorf("package %s location is the root directory", packageName)
		}

		if !sliceutil.IsStringInSlice(".pub-cache", pathElements) {
			log.Debugf("Flutter dependency cache: package not in system dependency cache: %s", location.Path)
			continue
		}

		// https://dart.dev/guides/libraries/create-library-packages
		if pathElements[len(pathElements)-1] != "lib" {
			log.Warnf("Flutter dependency cache: package path does not have top level 'lib' element: %s", location.Path)
			continue
		}

		// Package path the parent of "lib"
		cachePaths = append(cachePaths, filepath.Dir(location.Path))
	}

	return cachePaths, nil
}

func cacheFlutterDeps(projectDir string) error {
	contents, err := openPackageResolutionFile(projectDir)
	if err != nil {
		return err
	}

	packageToLocation, err := parsePackageResolutionFile(contents)
	if err != nil {
		return fmt.Errorf("failed to parse Flutter package resolution file, error: %s", err)
	}

	cachePaths, err := cacheableFlutterDepPaths(packageToLocation)
	if err != nil {
		return err
	}
	log.Debugf("Marking Flutter dependency paths to be cached: %s", cachePaths)

	pubCache := cache.New()
	for _, path := range cachePaths {
		pubCache.IncludePath(path)
	}
	return pubCache.Commit()
}
