# Flutter Build

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/bitrise-step-flutter-build?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/bitrise-step-flutter-build/releases)

Builds a Flutter project.

<details>
<summary>Description</summary>

This Step builds an iOS and an Android app. By default the Step builds the artifact based on the platform the scanner detects.

### Configuring the Step
1. In the **Project Location** input the root directory of your Flutter project is automatically filled out.
2. Select which platform your project should be built for (`ios`, `android` or `both`).
3. Enable **Debug** option to get verbose logs and see where the Step is failing.

Depending on the selected platform/s, continue with the rest of the config inputs.

#### Configuring for an iOS app
1. Make sure the **Platform input** is set to `iOS` or `both`.
2. In **Codesign Identity** you can onverride the code signing identities that you set in Flutter.
3. In **Additional parameters** add any flag to customize your build (for example, the `--release` flag appended to `flutter build io` builds a deployable iOS app).
4. Leave the **Output pattern** input's default value as is or modify it to the pattern if your build artifacts are stored elsewhere.
5. Make sure you have the **Xcode Archive & Export for iOS** Step after the **Flutter Build** Step in your Workflow.

#### Configuring for an Android app
1. Insert the **Android Sign** Step after the **Flutter Build** Step and make sure code signing files are uploaded to the **Code Signing** tab.
2. Make sure the **Platform input** is set to `Android` or `both`.
3. Scroll down to the `Android Platform Configs` input section, and select the preferred output artifact type you wish to generate in the **Android output artifact type** input. The Step can build an APK and an Android App Bundle as well.
4. Append any flag to the `build` command in the **Additional parameters** input.
5. Leave the **Output pattern** input's default value as is or modify it to the pattern if your build artifacts are stored elsewhere.

### Troubleshooting

Make sure the **Flutter Install** Step is before the **Flutter Build** Step.
If you have not set up code signing correctly, some code signing related issue will definitely surface by this build Step.
If you're unsure about code signing, consult our guide linked in Useful links.

### Useful links
- [Getting started with Flutter apps](https://devcenter.bitrise.io/getting-started/getting-started-with-flutter-apps/#deploying-a-flutter-app)
- [Available version tags](https://github.com/flutter/flutter/releases)
- [Available branches](https://github.com/flutter/flutter/branches)
- [Code signing](https://devcenter.bitrise.io/code-signing/code-signing-index/)

### Related Steps
- [Flutter Install](https://www.bitrise.io/integrations/steps/flutter-installer)
- [Flutter Test](https://www.bitrise.io/integrations/steps/flutter-test)
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/steps/adding-steps-to-a-workflow.html).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `project_location` | The root dir of your Flutter project. | required | `$BITRISE_SOURCE_DIR` |
| `platform` | The selected platform will be built, or both iOs and Android if you select both | required | `both` |
| `additional_build_params` | Additional params for flutter build.  Example: you can specify a Build Number for `flutter build` via the `--build-number` flutter build param. For example, to set it to the `$BITRISE_BUILD_NUMBER` you can set this input to: `--build-number=$BITRISE_BUILD_NUMBER`. |  |  |
| `is_debug_mode` | If debug mode is enabled, the step will print verbose logs | required | `false` |
| `cache_level` | If enabled, will cache: - pub packages - Android (gradle) cache - Carthage / Cocoapods dependencies | required | `all` |
| `ios_output_type` | Output type to build when building for iOS. Possible values: - `app`: Build an iOS application bundle via `flutter build ios` - `archive`: Build an iOS archive bundle via `flutter build ipa` | required | `app` |
| `ios_codesign_identity` | Override codesign identity in .flutter_settings |  |  |
| `ios_additional_params` | The flags from this input field will be appended to the `flutter build ios` command. |  | `--release` |
| `ios_output_pattern` | Separate patterns with a newline. | required | `*build/ios/iphoneos/*.app *build/ios/archive/*.xcarchive` |
| `android_output_type` | The selected output type will be build, either APK or app bundle (AAB) | required | `apk` |
| `android_additional_params` | The flags from this input field will be appended to the `flutter build apk` command. |  | `--release` |
| `android_output_pattern` | Will find the APK or AAB files - `depending on the build type input` - with the given pattern.<br/> Separate patterns with a newline. **Note**<br/> The step will export only the selected artifact type - `Android output artifact type` - even if the filter would accept other artifact types as well.  | required | `*build/app/outputs/apk/*/*.apk *build/app/outputs/bundle/*/*.aab` |
| `android_bundle_output_pattern` | Pattern to find built AAB artifacts relative to `$BITRISE_SOURCE_DIR` |  | `*build/app/outputs/bundle/*/*.aab` |
</details>

<details>
<summary>Outputs</summary>

| Environment Variable | Description |
| --- | --- |
| `BITRISE_APK_PATH` | The created .apk file's path. |
| `BITRISE_APK_PATH_LIST` | All created .apk file paths, separated by \|. |
| `BITRISE_APP_DIR_PATH` | The generated .app directory's path. |
| `BITRISE_XCARCHIVE_PATH` | The generated .xcarchive directory's path. |
| `BITRISE_XCARCHIVE_ZIP_PATH` | The generated .xcarchive directory compressed as a ZIP archive. |
| `BITRISE_AAB_PATH_LIST` | This output will include the paths of the generated AAB files, after filtering based on the filter inputs. The paths are separated with `\|` character, eg: `app.aab\|app2.aab` |
| `BITRISE_AAB_PATH` | This output will include the path of the generated AAB file, after filtering based on the filter inputs. If the build generates more than one AAB file which fulfills the filter inputs this output will contain the last one's path. |
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/bitrise-step-flutter-build/pulls) and [issues](https://github.com/bitrise-steplib/bitrise-step-flutter-build/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://docs.bitrise.io/en/bitrise-ci/bitrise-cli/running-your-first-local-build-with-the-cli.html).

Learn more about developing steps:

- [Create your own step](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/developing-your-own-bitrise-step/developing-a-new-step.html)
