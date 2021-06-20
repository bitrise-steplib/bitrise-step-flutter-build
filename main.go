package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-xcode/certificateutil"
	shellquote "github.com/kballard/go-shellquote"
)

// AndroidArtifactType is an enum
// **APK** or **AppBundle**
type AndroidArtifactType string

// const ...
const (
	codesignField  = "ios-signing-cert"
	noCodesignFlag = "--no-codesign"

	APK       AndroidArtifactType = "apk"
	AppBundle AndroidArtifactType = "appbundle"
)

var flutterConfigPath = filepath.Join(os.Getenv("HOME"), ".flutter_settings")
var errCodeSign = errors.New("CODESIGN")

type config struct {
	AdditionalBuildParams   string              `env:"additional_build_params"`
	IOSAdditionalParams     string              `env:"ios_additional_params"`
	AndroidAdditionalParams string              `env:"android_additional_params"`
	Platform                string              `env:"platform,opt[both,ios,android]"`
	IOSExportPattern        string              `env:"ios_output_pattern,required"`
	AndroidOutputType       AndroidArtifactType `env:"android_output_type,opt[apk,appbundle]"`
	AndroidExportPattern    string              `env:"android_output_pattern,required"`
	IOSCodesignIdentity     string              `env:"ios_codesign_identity"`
	ProjectLocation         string              `env:"project_location,dir"`
	DebugMode               bool                `env:"is_debug_mode,opt[true,false]"`
	CacheLevel              string              `env:"cache_level,opt[all,none]"`

	// Deprecated
	AndroidBundleExportPattern string `env:"android_bundle_output_pattern"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func handleDeprecatedInputs(cfg *config) {
	if cfg.AndroidBundleExportPattern != "" && cfg.AndroidBundleExportPattern != "*build/app/outputs/bundle/*/*.aab" {
		log.Warnf("step input 'App bundle output pattern' (android_bundle_output_pattern) is deprecated and will be removed on 20 November 2019, use 'Output (.apk, .aab) pattern' (android_output_pattern) instead!")
		log.Printf("Using 'App bundle output pattern' (android_bundle_output_pattern) instead of 'Output (.apk, .aab) pattern' (android_output_pattern).")
		log.Printf("If you don't want to use 'App bundle output pattern' (android_bundle_output_pattern), empty it's value.")

		cfg.AndroidExportPattern = cfg.AndroidBundleExportPattern
	}
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(cfg)
	handleDeprecatedInputs(&cfg)
	log.SetEnableDebugLog(cfg.DebugMode)

	projectLocationAbs, err := filepath.Abs(cfg.ProjectLocation)
	if err != nil {
		failf("Failed to get absolute project path, error: %s", err)
	}

	exist, err := pathutil.IsDirExists(projectLocationAbs)
	if err != nil {
		failf("Failed to check if project path exists, error: %s", err)
	} else if !exist {
		failf("Project path does not exist.")
	}

	if cfg.Platform == "ios" || cfg.Platform == "both" {
		fmt.Println()
		log.Infof("iOS Codesign settings")

		iosParams, err := shellquote.Split(cfg.IOSAdditionalParams)
		if err != nil {
			failf(" - Failed to get iOS additional parameters, error: %s", err)
		}
		if sliceutil.IsStringInSlice(noCodesignFlag, iosParams) {
			log.Printf(" - Skipping codesign preparation, %s parameter set", noCodesignFlag)
			goto build
		}

		log.Printf(" Installed codesign identities:")
		installedCertificates, err := certificateutil.InstalledCodesigningCertificateNames()
		if err != nil {
			failf(" - Failed to fetch installed codesign identities, error: %s", err)
		}
		for _, identity := range installedCertificates {
			log.Printf(" - %s", identity)
		}

		if len(installedCertificates) == 0 {
			failf(" - No codesign identities installed")
		}

		var flutterSettings map[string]string
		flutterSettingsExists, err := pathutil.IsPathExists(flutterConfigPath)
		if err != nil {
			failf(" - Failed to check if path exists, error: %s", err)
		}
		if flutterSettingsExists {
			flutterSettingsContent, err := fileutil.ReadBytesFromFile(flutterConfigPath)
			if err != nil {
				failf(" - Failed to check if path exists, error: %s", err)
			}
			if err := json.Unmarshal(flutterSettingsContent, &flutterSettings); err != nil {
				failf(" - Failed to unmarshal .flutter_settings file, error: %s", err)
			}
		} else {
			flutterSettings = map[string]string{}
		}

		if cfg.IOSCodesignIdentity != "" {
			log.Warnf(" Override codesign identity:")
			log.Printf(" - Store: %s", cfg.IOSCodesignIdentity)
			if !sliceutil.IsStringInSlice(cfg.IOSCodesignIdentity, installedCertificates) {
				failf(" - The selected identity \"%s\" is not installed on the system", cfg.IOSCodesignIdentity)
			}
			flutterSettings[codesignField] = cfg.IOSCodesignIdentity
			newSettingsContent, err := json.MarshalIndent(flutterSettings, "", " ")
			if err != nil {
				failf(" - Failed to unmarshal .flutter_settings file, error: %s", err)
			}
			if err := fileutil.WriteBytesToFile(flutterConfigPath, newSettingsContent); err != nil {
				failf(" - Failed to write .flutter_settings file, error: %s", err)
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
				failf(" - Identity \"%s\" is not installed on the system", storedIdentity)
			}
		}
	}

build:

	for _, spec := range []buildSpecification{
		{
			displayName:          "iOS",
			platformCmdFlag:      "ios",
			platformSelectors:    []string{"both", "ios"},
			outputPathPatterns:   strings.Split(cfg.IOSExportPattern, "\n"),
			additionalParameters: cfg.AdditionalBuildParams + " " + cfg.IOSAdditionalParams,
		},
		{
			displayName:          "Android",
			platformCmdFlag:      string(cfg.AndroidOutputType),
			platformSelectors:    []string{"both", "android"},
			outputPathPatterns:   strings.Split(cfg.AndroidExportPattern, "\n"),
			additionalParameters: cfg.AdditionalBuildParams + " " + cfg.AndroidAdditionalParams,
		},
	} {
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

			failf("Failed to build %s platform, error: %s", spec.displayName, err)
		}

		fmt.Println()
		log.Infof("Export " + spec.displayName + " artifact")

		var artifacts []string
		var err error

		if spec.platformCmdFlag == "apk" || spec.platformCmdFlag == "appbundle" {
			artifacts, err = spec.artifactPaths(spec.outputPathPatterns, false)
		} else {
			artifacts, err = spec.artifactPaths(spec.outputPathPatterns, true)
		}

		if err != nil {
			failf("failed to find artifacts, error: %s", err)
		}

		if len(artifacts) < 1 {
			failf(`Artifact path pattern (%s) did not match any artifacts on the path (%s).
Check that 'iOS/Android Output Pattern' and 'Project Location' is correct.`, spec.outputPathPatterns, spec.projectLocation)
		}

		if err := spec.exportArtifacts(artifacts); err != nil {
			failf("Failed to export %s artifacts, error: %s", spec.displayName, err)
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
