package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	androidCache "github.com/bitrise-io/go-android/cache"
	"github.com/bitrise-io/go-steputils/cache"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
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

	return androidCache.Collect(androidDir, cache.LevelDeps)
}

func openFile(filepath string) (string, error) {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return "", fmt.Errorf("file (%s) not found, error: %s", filepath, err)
	}

	contents, err := ioutil.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read package resolution file, error: %s", err)
	}

	return string(contents), nil
}

// parsePackageResolutionFile parses flutter package resolution file: `.package`
// https://dart.dev/tools/pub/cmd/pub-get
/* If there are any packages from git source the whole `git` directory is cached,
as the contents of `.git` dir may be needed for package resolution.
The layout of .pub-cache:
.pub-cache
|- global_packages
    |- junitreport
        …
    …
|- bin
    |- tojunit
    …
|- git     									→ packages from git source
     |- cache
          |- <package_name>-<commit_hash>   →  A copy of the .git dir
               |- refs
               …
          |- <package_name2>-<commit_hash2> →  An other copy of the .git dir
          …
     |- <package_name>-<commit_hash>  		→  A checked out package
          |- .git
          |- lib
          …
     |- <package_name2>-<commit_hash2>		→  An other checked out package
          |- .git
              |- .pub-packages   			→  Needs to be cached
          |- mypath
              |- lib       					→  The resolved source code path for packages from git
         …
|- hosted    								→  Hosted packages
    |- pub.dartlang.org
        |- async-2.4.1
            |- lib							→  The resolved package source code path for hosted packages
        …
*/
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
	foundGitSourcePackages := false

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

		cacheRootIndex := sliceutil.IndexOfStringInSlice(".pub-cache", pathElements)
		if cacheRootIndex == -1 {
			log.Debugf("Flutter dependency cache: package not in system dependency cache: %s", location.Path)
			continue
		}

		// https://dart.dev/guides/libraries/create-library-packages
		if pathElements[len(pathElements)-1] != "lib" {
			log.Warnf("Flutter dependency cache: package path does not have top level 'lib' element: %s", location.Path)
			continue
		}

		gitRootIndex := cacheRootIndex + 1
		if len(pathElements) > gitRootIndex+1 && pathElements[gitRootIndex] == "git" {
			log.Debugf("Flutter dependency cache: found pub package with git source: %s", location.Path)
			if !foundGitSourcePackages {
				foundGitSourcePackages = true
				// Append git packages root path, e.g $HOME/.pub-cache/git
				// (.pub-cache dir can be located in $HOME or in the Flutter SDK dir.)
				gitRoot := location.Path
				for ; gitRootIndex < len(pathElements)-1; gitRootIndex++ {
					gitRoot = filepath.Dir(gitRoot)
				}
				cachePaths = append(cachePaths, gitRoot)
			}
			continue
		}

		// Package path the parent of "lib"
		cachePaths = append(cachePaths, filepath.Dir(location.Path))
	}

	return cachePaths, nil
}

func cacheFlutterDeps(projectDir string) error {
	packageToLocation, err := readOldPackageFormat(projectDir)
	if err != nil {
		packageToLocation, err = readNewJSONFormat(projectDir)
		if err != nil {
			return err
		}
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

func readOldPackageFormat(projectDir string) (map[string]url.URL, error) {
	packagePath := filepath.Join(projectDir, ".packages")
	contents, err := openFile(packagePath)
	if err != nil {
		return map[string]url.URL{}, err
	}

	packageToLocation, err := parsePackageResolutionFile(contents)
	if err != nil {
		return map[string]url.URL{}, fmt.Errorf("failed to parse Flutter package resolution file, error: %s", err)
	}

	return packageToLocation, nil
}

func readNewJSONFormat(projectDir string) (map[string]url.URL, error) {
	packagePath := filepath.Join(projectDir, ".dart_tool/package_config.json")
	contents, err := openFile(packagePath)
	if err != nil {
		return map[string]url.URL{}, err
	}

	packages, err := parseJSON(contents)
	if err != nil {
		return map[string]url.URL{}, err
	}

	return packages, nil
}

func parseJSON(contents string) (map[string]url.URL, error) {
	type packageConfig struct {
		Packages []struct {
			Name       string `json:"name"`
			RootUri    string `json:"rootUri"`
			PackageUri string `json:"packageUri"`
		} `json:"packages"`
	}

	var config packageConfig
	err := json.Unmarshal([]byte(contents), &config)
	if err != nil {
		return map[string]url.URL{}, err
	}

	packages := map[string]url.URL{}

	for _, item := range config.Packages {
		path := filepath.Join(item.RootUri, item.PackageUri)
		location, err := url.Parse(path)
		if err != nil {
			return map[string]url.URL{}, fmt.Errorf("could not parse location URI: %s", path)
		}

		packages[item.Name] = *location
	}

	return packages, nil
}
