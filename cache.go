package main

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/cache"
	"github.com/bitrise-io/go-utils/pathutil"
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
	androidDir, err := pathutil.AbsPath(filepath.Join(projectDir, "android"))
	if err != nil {
		return err
	}

	return androidCache.Collect(androidDir, androidCache.LevelDeps)
}
