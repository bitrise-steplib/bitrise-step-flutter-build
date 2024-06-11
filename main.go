package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-xcode/certificateutil"
	shellquote "github.com/kballard/go-shellquote"
)

type OutputType string

const (
	codesignField  = "ios-signing-cert"
	noCodesignFlag = "--no-codesign"

	OutputTypeAPK       OutputType = "apk"
	OutputTypeAppBundle OutputType = "appbundle"

	OutputTypeIOSApp             OutputType = "app"               // CLI: flutter build ios
	OutputTypeIOSAppWithCodeSign OutputType = "app-with-codesign" // CLI: flutter build ios + codesign
	OutputTypeArchive            OutputType = "archive"           // CLI: flutter build ipa
)

var flutterConfigPath = filepath.Join(os.Getenv("HOME"), ".flutter_settings")
var errCodeSign = errors.New("CODESIGN")

type config struct {
	ProjectLocation       string `env:"project_location,dir"`
	Platform              string `env:"platform,opt[both,ios,android]"`
	AdditionalBuildParams string `env:"additional_build_params"`
	DebugMode             bool   `env:"is_debug_mode,opt[true,false]"`
	CacheLevel            string `env:"cache_level,opt[all,none]"`

	IOSOutputType       OutputType `env:"ios_output_type,opt[app,app-with-codesign,archive]"`
	IOSAdditionalParams string     `env:"ios_additional_params"`
	IOSExportPattern    []string   `env:"ios_output_pattern,multiline"`
	IOSCodesignIdentity string     `env:"ios_codesign_identity"`

	AndroidOutputType       OutputType `env:"android_output_type,opt[apk,appbundle]"`
	AndroidAdditionalParams string     `env:"android_additional_params"`
	AndroidExportPattern    []string   `env:"android_output_pattern,multiline"`

	// Deprecated
	AndroidBundleExportPattern []string `env:"android_bundle_output_pattern,multiline"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func handleDeprecatedInputs(cfg *config) {
	if len(cfg.AndroidBundleExportPattern) > 0 && cfg.AndroidBundleExportPattern[0] != "*build/app/outputs/bundle/*/*.aab" {
		log.Warnf("step input 'App bundle output pattern' (android_bundle_output_pattern) is deprecated and will be removed on 20 November 2019, use 'Output (.apk, .aab) pattern' (android_output_pattern) instead!")
		log.Printf("Using 'App bundle output pattern' (android_bundle_output_pattern) instead of 'Output (.apk, .aab) pattern' (android_output_pattern).")
		log.Printf("If you don't want to use 'App bundle output pattern' (android_bundle_output_pattern), empty it's value.")

		cfg.AndroidExportPattern = cfg.AndroidBundleExportPattern
	}
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Process config: failed to parse input: %s", err)
	}
	stepconf.Print(cfg)
	handleDeprecatedInputs(&cfg)
	log.SetEnableDebugLog(cfg.DebugMode)

	projectLocationAbs, err := filepath.Abs(cfg.ProjectLocation)
	if err != nil {
		failf("Process config: failed to get absolute project path of %s: %s", cfg.ProjectLocation, err)
	}

	exist, err := pathutil.IsDirExists(projectLocationAbs)
	if err != nil {
		failf("Process config: failed to check if project path exists: %s", err)
	} else if !exist {
		failf("Process config: project path does not exist")
	}

	if cfg.Platform == "ios" || cfg.Platform == "both" {
		fmt.Println()
		log.Infof("iOS Codesign settings")

		iosParams, err := shellquote.Split(cfg.IOSAdditionalParams)
		if err != nil {
			failf("Process config: failed to parse iOS additional parameters: %s", err)
		}
		if sliceutil.IsStringInSlice(noCodesignFlag, iosParams) {
			log.Printf(" - Skipping codesign preparation, %s parameter set", noCodesignFlag)
			goto build
		}
		if cfg.IOSOutputType == OutputTypeIOSApp {
			log.Printf(" - Skipping codesign preparation because output type is iOS app, not xcarchive")
			goto build
		}

		log.Printf(" Installed codesign identities:")
		installedCertificates, err := certificateutil.InstalledCodesigningCertificateNames()
		if err != nil {
			failf("Run: failed to fetch installed codesign identities: %s", err)
		}
		for _, identity := range installedCertificates {
			log.Printf(" - %s", identity)
		}

		if len(installedCertificates) == 0 {
			failf("Run: no codesign identities installed")
		}

		var flutterSettings map[string]string
		flutterSettingsExists, err := pathutil.IsPathExists(flutterConfigPath)
		if err != nil {
			failf("Run: failed to check if %s exists: %s", flutterConfigPath, err)
		}
		if flutterSettingsExists {
			flutterSettingsContent, err := fileutil.ReadBytesFromFile(flutterConfigPath)
			if err != nil {
				failf("Run: error while reading %s: %s", flutterConfigPath, err)
			}
			if err := json.Unmarshal(flutterSettingsContent, &flutterSettings); err != nil {
				failf("Run: failed to parse .flutter_settings file: %s", err)
			}
		} else {
			flutterSettings = map[string]string{}
		}

		if cfg.IOSCodesignIdentity != "" {
			log.Warnf(" Override codesign identity:")
			log.Printf(" - Store: %s", cfg.IOSCodesignIdentity)
			if !sliceutil.IsStringInSlice(cfg.IOSCodesignIdentity, installedCertificates) {
				failf("Process config: the selected identity \"%s\" is not installed on the system", cfg.IOSCodesignIdentity)
			}
			flutterSettings[codesignField] = cfg.IOSCodesignIdentity
			newSettingsContent, err := json.MarshalIndent(flutterSettings, "", " ")
			if err != nil {
				failf("Run: failed to parse .flutter_settings file: %s", err)
			}
			if err := fileutil.WriteBytesToFile(flutterConfigPath, newSettingsContent); err != nil {
				failf("Run: error while writing .flutter_settings file: %s", err)
			}
			log.Donef(" - Done")
			goto build
		}

		log.Printf(" Stored Flutter codesign settings:")
		storedIdentity, ok := flutterSettings["ios-signing-cert"]
		if !ok {
			log.Printf(" - No codesign identity set")
		} else {
			log.Printf(" - %s", storedIdentity)
			if !sliceutil.IsStringInSlice(storedIdentity, installedCertificates) {
				failf("Process config: identity \"%s\" is not installed on the system", storedIdentity)
			}
		}
	}

build:

	buildSpecifications := []buildSpecification{
		{
			displayName:          "iOS app",
			platformOutputType:   cfg.IOSOutputType,
			platformSelectors:    []string{"both", "ios"},
			outputPathPatterns:   cfg.IOSExportPattern,
			additionalParameters: cfg.AdditionalBuildParams + " " + cfg.IOSAdditionalParams,
		},
		{
			displayName:          "Android app",
			platformOutputType:   cfg.AndroidOutputType,
			platformSelectors:    []string{"both", "android"},
			outputPathPatterns:   cfg.AndroidExportPattern,
			additionalParameters: cfg.AdditionalBuildParams + " " + cfg.AndroidAdditionalParams,
		},
	}

	for _, spec := range buildSpecifications {
		if !spec.buildable(cfg.Platform) {
			continue
		}

		spec.projectLocation = projectLocationAbs

		fmt.Println()
		log.Infof("Build " + spec.displayName)
		if err := spec.build(spec.additionalParameters); err != nil {
			if err == errCodeSign {
				if cfg.IOSCodesignIdentity != "" {
					log.Warnf("Invalid codesign identity is selected, choose the appropriate identity in the step's [iOS Platform Configs>Codesign Identity] input field.")
				} else {
					log.Warnf("You have multiple codesign identity installed, select the one you want to use and set its name in the [iOS Platform Configs>Codesign Identity] input field.")
				}
			}

			failf("Run: failed to build %s: %s", spec.displayName, err)
		}

		fmt.Println()
		log.Infof("Export " + spec.displayName + " artifact")

		var artifacts []string
		var err error

		if spec.platformOutputType == OutputTypeAPK || spec.platformOutputType == OutputTypeAppBundle {
			artifacts, err = spec.artifactPaths(spec.outputPathPatterns, false)
		} else {
			artifacts, err = spec.artifactPaths(spec.outputPathPatterns, true)
		}

		if err != nil {
			failf("Export outputs: failed to find artifacts: %s", err)
		}

		if len(artifacts) < 1 {
			failf(`Export outputs: artifact path pattern (%s) did not match any artifacts on the path (%s).
Check that 'iOS/Android Output Pattern' and 'Project Location' is correct.`, spec.outputPathPatterns, spec.projectLocation)
		}

		if err := spec.exportArtifacts(artifacts); err != nil {
			failf("Export outputs: failed to export %s artifacts: %s", spec.displayName, err)
		}
	}

	if cfg.CacheLevel == "all" {
		fmt.Println()
		log.Infof("Collecting cache")

		if err := cacheCocoapodsDeps(projectLocationAbs); err != nil {
			log.Warnf("Failed to collect cocoapods cache, error: %s", err)
		}

		if err := cacheCarthageDeps(projectLocationAbs); err != nil {
			log.Warnf("Failed to collect carthage cache, error: %s", err)
		}

		if err := cacheAndroidDeps(projectLocationAbs); err != nil {
			log.Warnf("Failed to collect android cache, error: %s", err)
		}

		if err := cacheFlutterDeps(projectLocationAbs); err != nil {
			log.Warnf("Failed to collect flutter cache, error: %s", err)
		}
	}
}
