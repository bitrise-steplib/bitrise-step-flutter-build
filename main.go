package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"github.com/bitrise-tools/go-xcode/certificateutil"
	shellquote "github.com/kballard/go-shellquote"
)

const (
	codesignField  = "ios-signing-cert"
	noCodesignFlag = "--no-codesign"
)

var flutterConfigPath = filepath.Join(os.Getenv("HOME"), ".flutter_settings")
var errCodeSign = errors.New("CODESIGN")

type config struct {
	IOSAdditionalParams     string `env:"ios_additional_params"`
	AndroidAdditionalParams string `env:"android_additional_params"`
	Platform                string `env:"platform,opt[both,ios,android]"`
	IOSExportPattern        string `env:"ios_output_pattern"`
	AndroidExportPattern    string `env:"android_output_pattern"`
	IOSCodesignIdentity     string `env:"ios_codesign_identity"`
	ProjectLocation         string `env:"project_location,dir"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(cfg)

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
		buildSpecification{
			displayName:          "iOS",
			platformCmdFlag:      "ios",
			platformSelectors:    []string{"both", "ios"},
			outputPathPattern:    cfg.IOSExportPattern,
			additionalParameters: cfg.IOSAdditionalParams,
		},
		buildSpecification{
			displayName:          "Android",
			platformCmdFlag:      "apk",
			platformSelectors:    []string{"both", "android"},
			outputPathPattern:    cfg.AndroidExportPattern,
			additionalParameters: cfg.AndroidAdditionalParams,
		},
	} {
		if !spec.buildable(cfg.Platform) {
			continue
		}

		spec.projectLocation = cfg.ProjectLocation

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

		if err := spec.exportArtifacts(spec.outputPathPattern); err != nil {
			failf("Failed to export %s artifacts, error: %s", spec.displayName, err)
		}
	}
}
