// Copyright (c) 2018-2021 Splunk Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.s
package s1appfw

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	testenv "github.com/splunk/splunk-operator/test/testenv"

	enterpriseApi "github.com/splunk/splunk-operator/pkg/apis/enterprise/v3"
	splcommon "github.com/splunk/splunk-operator/pkg/splunk/common"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("s1appfw test", func() {

	var deployment *testenv.Deployment
	var s3TestDir string
	var uploadedApps []string

	BeforeEach(func() {
		var err error
		deployment, err = testenvInstance.NewDeployment(testenv.RandomDNSName(3))
		Expect(err).To(Succeed(), "Unable to create deployment")
	})

	AfterEach(func() {
		// When a test spec failed, skip the teardown so we can troubleshoot.
		if CurrentGinkgoTestDescription().Failed {
			testenvInstance.SkipTeardown = true
		}
		if deployment != nil {
			deployment.Teardown()
		}
		// Delete files uploaded to S3
		if !testenvInstance.SkipTeardown {
			testenv.DeleteFilesOnS3(testS3Bucket, uploadedApps)
		}
	})

	Context("Standalone deployment (S1) with App Framework", func() {
		It("smoke, s1, appframework: can deploy a Standalone instance with App Framework enabled, install apps then upgrade them", func() {

			/* Test Steps
			   ################## SETUP ####################
			   * Upload V1 apps to S3 for Monitoring Console
			   * Create app source for Monitoring Console
			   * Prepare and deploy Monitoring Console with app framework and wait for the pod to be ready
			   * Upload V1 apps to S3 for Standalone
			   * Create app source for Standalone
			   * Prepare and deploy Standalone with app framework and wait for the pod to be ready
			   ############ V1 APP VERIFICATION FOR STANDALONE AND MONITORING CONSOLE ###########
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			   ############ UPGRADE V2 APPS ###########
			   * Upload V2 apps to S3 App Source
			   ############ V2 APP VERIFICATION FOR STANDALONE AND MONITORING CONSOLE  ###########
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			*/

			// ################## SETUP FOR MONITORING CONSOLE ####################

			// Upload V1 apps to S3 for Monitoring Console
			appVersion := "V1"
			appFileList := testenv.GetAppFileListPhase3(appListV1)
			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Monitoring Console", appVersion))
			s3TestDirMC := "s1appfw-mc-" + testenv.RandomDNSName(4)
			uploadedFiles, err := testenv.UploadFilesToS3(testS3Bucket, s3TestDirMC, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Monitoring Console", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework spec for Monitoring Console
			volumeNameMC := "appframework-test-volume-mc-" + testenv.RandomDNSName(3)
			volumeSpecMC := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeNameMC, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}
			appSourceDefaultSpecMC := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeNameMC,
				Scope:   enterpriseApi.ScopeLocal,
			}
			appSourceNameMC := "appframework-mc-" + testenv.RandomDNSName(3)
			appSourceSpecMC := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceNameMC, s3TestDirMC, appSourceDefaultSpecMC)}
			appFrameworkSpecMC := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpecMC,
				AppsRepoPollInterval: 60,
				VolList:              volumeSpecMC,
				AppSources:           appSourceSpecMC,
			}
			mcSpec := enterpriseApi.MonitoringConsoleSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "IfNotPresent",
					},
					Volumes: []corev1.Volume{},
				},
				AppFrameworkConfig: appFrameworkSpecMC,
			}

			// Deploy Monitoring Console
			testenvInstance.Log.Info("Deploy Monitoring Console")
			mcName := deployment.GetName()
			mc, err := deployment.DeployMonitoringConsoleWithGivenSpec(testenvInstance.GetName(), mcName, mcSpec)
			Expect(err).To(Succeed(), "Unable to deploy Monitoring Console")

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			// ################## SETUP FOR STANDALONE ####################

			// Upload V1 apps to S3 for Standalone
			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Standalone", appVersion))
			s3TestDir = "s1appfw-" + testenv.RandomDNSName(4)
			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Standalone", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework spec for Standalone
			volumeName := "appframework-test-volume-" + testenv.RandomDNSName(3)
			volumeSpec := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeName, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}
			appSourceDefaultSpec := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeName,
				Scope:   enterpriseApi.ScopeLocal,
			}

			// Maximum apps to be downloaded in parallel
			maxConcurrentAppDownloads := 5

			// appSourceSpec: App source name, location and volume name and scope from appSourceDefaultSpec
			appSourceName := "appframework" + testenv.RandomDNSName(3)
			appSourceSpec := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceName, s3TestDir, appSourceDefaultSpec)}
			appFrameworkSpec := enterpriseApi.AppFrameworkSpec{
				Defaults:                  appSourceDefaultSpec,
				AppsRepoPollInterval:      60,
				VolList:                   volumeSpec,
				AppSources:                appSourceSpec,
				MaxConcurrentAppDownloads: uint64(maxConcurrentAppDownloads),
			}
			spec := enterpriseApi.StandaloneSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "Always",
					},
					Volumes: []corev1.Volume{},
					MonitoringConsoleRef: corev1.ObjectReference{
						Name: mcName,
					},
				},
				AppFrameworkConfig: appFrameworkSpec,
			}

			// Deploy Standalone
			testenvInstance.Log.Info("Deploy Standalone")
			standalone, err := deployment.DeployStandaloneWithGivenSpec(deployment.GetName(), spec)
			Expect(err).To(Succeed(), "Unable to deploy Standalone instance with App framework")

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			// ############ INITIAL VERIFICATION ###########

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Download State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify App Copy State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			opLocalAppPathStandalone := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), standalone.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			opPod := testenv.GetOperatorPodName(testenvInstance.GetName())
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify Apps Deleted on Operator Pod for Monitoring Console
			opLocalAppPathMonitoringConsole := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), mc.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathMonitoringConsole)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify App Install State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify V1 apps are deleted on Standalone Pod
			downloadLocationStandalonePod := "/init-apps/" + appSourceName
			standalonePodName := fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify V1 apps are deleted on Monitoring Console Pod
			downloadLocationMCPod := "/init-apps/" + appSourceNameMC
			mcPodName := fmt.Sprintf(testenv.MonitoringConsolePod, mcName, 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Monitoring Console pod %s", appVersion, mcPodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{mcPodName}, appFileList, downloadLocationMCPod)

			// Verify V1 apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on Standalone and Monitoring Console", appVersion))
			podNames := []string{standalonePodName, mcPodName}
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, false)

			// Verify V1 apps are installed
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are installed on Standalone and Monitoring Console", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, "enabled", false, false)

			// ############## UPGRADE APPS #################
			// Delete apps on S3
			testenvInstance.Log.Info(fmt.Sprintf("Delete %s apps on S3", appVersion))
			testenv.DeleteFilesOnS3(testS3Bucket, uploadedApps)
			uploadedApps = nil

			// Upload V2 apps to S3 for Standalone and Monitoring Console
			appVersion = "V2"
			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Standalone and Monitoring Console", appVersion))
			appFileList = testenv.GetAppFileListPhase3(appListV2)

			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV2)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Standalone", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDirMC, appFileList, downloadDirV2)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Monitoring Console", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Wait for the poll period for the apps to be downloaded
			time.Sleep(2 * time.Minute)

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			//############ UPGRADE VERIFICATION ###########

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Download State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify App Copy State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify Apps Deleted on Operator Pod for Monitoring Console
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathMonitoringConsole)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify App Install State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify V1 apps are deleted on Standalone Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify V1 apps are deleted on Monitoring Console Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Monitoring Console pod %s", appVersion, mcPodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{mcPodName}, appFileList, downloadLocationMCPod)

			// Verify V2 apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on Standalone and Monitoring Console", appVersion))
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV2, true, false)

			// Verify V2 apps are installed
			testenvInstance.Log.Info(fmt.Sprintf("Verify apps have been updated to %s on Standalone and Monitoring Console", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV2, true, "enabled", true, false)

		})
	})

	Context("Standalone deployment (S1) with App Framework", func() {
		It("smoke, s1, appframework: can deploy a Standalone instance with App Framework enabled, install apps then downgrade them", func() {

			/* Test Steps
			   ################## SETUP ####################
			   * Upload V2 apps to S3 for Monitoring Console
			   * Create app source for Monitoring Console
			   * Prepare and deploy Monitoring Console with app framework and wait for the pod to be ready
			   * Upload V2 apps to S3 for Standalone
			   * Create app source for Standalone
			   * Prepare and deploy Standalone with app framework and wait for the pod to be ready
			   ############ INITIAL VERIFICATION FOR STANDALONE AND MONITORING CONSOLE ###########
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			   ############# DOWNGRADE APPS ################
			   * Upload V1 apps on S3
			   * Wait for Monitoring Console and Standalone pods to be ready
			   ########## DOWNGRADE VERIFICATION FOR STANDALONE AND MONITORING CONSOLE ###########
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			*/

			//################## SETUP ####################
			// Upload V2 apps to S3
			appVersion := "V2"
			appFileList := testenv.GetAppFileListPhase3(appListV2)
			s3TestDir = "s1appfw-" + testenv.RandomDNSName(4)

			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Standalone", appVersion))
			uploadedFiles, err := testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV2)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Standalone", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Monitoring Console", appVersion))
			s3TestDirMC := "s1appfw-mc-" + testenv.RandomDNSName(4)
			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDirMC, appFileList, downloadDirV2)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Monitoring Console", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework Spec for Monitoring Console
			volumeNameMC := "appframework-test-volume-mc-" + testenv.RandomDNSName(3)
			volumeSpecMC := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeNameMC, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}
			appSourceDefaultSpecMC := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeNameMC,
				Scope:   enterpriseApi.ScopeLocal,
			}
			appSourceNameMC := "appframework-mc-" + testenv.RandomDNSName(3)
			appSourceSpecMC := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceNameMC, s3TestDirMC, appSourceDefaultSpecMC)}
			appFrameworkSpecMC := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpecMC,
				AppsRepoPollInterval: 60,
				VolList:              volumeSpecMC,
				AppSources:           appSourceSpecMC,
			}
			mcSpec := enterpriseApi.MonitoringConsoleSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "IfNotPresent",
					},
					Volumes: []corev1.Volume{},
				},
				AppFrameworkConfig: appFrameworkSpecMC,
			}

			// Deploy Monitoring Console
			testenvInstance.Log.Info("Deploy Monitoring Console")
			mcName := deployment.GetName()
			mc, err := deployment.DeployMonitoringConsoleWithGivenSpec(testenvInstance.GetName(), mcName, mcSpec)
			Expect(err).To(Succeed(), "Unable to deploy Monitoring Console")

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			// Create App framework Spec for Standalone
			volumeName := "appframework-test-volume-" + testenv.RandomDNSName(3)
			volumeSpec := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeName, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}
			appSourceDefaultSpec := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeName,
				Scope:   enterpriseApi.ScopeLocal,
			}
			appSourceName := "appframework" + testenv.RandomDNSName(3)
			appSourceSpec := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceName, s3TestDir, appSourceDefaultSpec)}
			appFrameworkSpec := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpec,
				AppsRepoPollInterval: 60,
				VolList:              volumeSpec,
				AppSources:           appSourceSpec,
			}
			spec := enterpriseApi.StandaloneSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "Always",
					},
					Volumes: []corev1.Volume{},
					MonitoringConsoleRef: corev1.ObjectReference{
						Name: mcName,
					},
				},
				AppFrameworkConfig: appFrameworkSpec,
			}

			// Deploy Standalone
			testenvInstance.Log.Info("Deploy Standalone")
			standalone, err := deployment.DeployStandaloneWithGivenSpec(deployment.GetName(), spec)
			Expect(err).To(Succeed(), "Unable to deploy Standalone instance with App framework")

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			//############ INITIAL VERIFICATION ###########

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Download State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify App Copy State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			opLocalAppPathStandalone := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), standalone.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			opPod := testenv.GetOperatorPodName(testenvInstance.GetName())
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify Apps Deleted on Operator Pod for Monitoring Console
			opLocalAppPathMonitoringConsole := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), mc.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathMonitoringConsole)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify App Install State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify V1 apps are deleted on Standalone Pod
			downloadLocationStandalonePod := "/init-apps/" + appSourceName
			standalonePodName := fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify V1 apps are deleted on Monitoring Console Pod
			downloadLocationMCPod := "/init-apps/" + appSourceNameMC
			mcPodName := fmt.Sprintf(testenv.MonitoringConsolePod, mcName, 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Monitoring Console pod %s", appVersion, mcPodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{mcPodName}, appFileList, downloadLocationMCPod)

			// Verify V2 apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on Standalone and Monitoring Console", appVersion))
			podNames := []string{standalonePodName, mcPodName}
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV2, true, false)

			// Verify V2 apps are installed
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are installed on Standalone and Monitoring Console", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV2, true, "enabled", true, false)

			// ############# DOWNGRADE APPS ################
			// Delete apps on S3
			testenvInstance.Log.Info(fmt.Sprintf("Delete %s apps on S3", appVersion))
			testenv.DeleteFilesOnS3(testS3Bucket, uploadedApps)
			uploadedApps = nil

			// Upload V1 apps to S3 for Standalone and Monitoring Console
			appVersion = "V1"
			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Standalone and Monitoring Console", appVersion))
			appFileList = testenv.GetAppFileListPhase3(appListV1)

			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Standalone", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDirMC, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Monitoring Console", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Wait for the poll period for the apps to be downloaded
			time.Sleep(2 * time.Minute)

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			//########## DOWNGRADE VERIFICATION ###########

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Download State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify App Copy State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify Apps Deleted on Operator Pod for Monitoring Console
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathMonitoringConsole)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify App Install State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify apps are deleted on Standalone Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify apps are deleted on Monitoring Console Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Monitoring Console pod %s", appVersion, mcPodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{mcPodName}, appFileList, downloadLocationMCPod)

			// Verify apps are copied to correct location on splunk pods
			testenvInstance.Log.Info("Verify Apps are copied to correct location on Pod for app", "version", appVersion)
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, true)

			// Verify Apps are installed
			testenvInstance.Log.Info("Verify Apps are installed on the pods by running Splunk CLI commands for app", "version", appVersion)
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, "enabled", false, false)
		})
	})

	Context("Standalone deployment (S1) with App Framework", func() {
		It("s1, integration, appframework: can deploy a Standalone instance with App Framework enabled, install apps, scale up, install apps on new pod, scale down", func() {

			/* Test Steps
			   ################## SETUP ####################
			   * Upload apps on S3
			   * Create 2 app sources for Monitoring Console and Standalone
			   * Prepare and deploy Monitoring Console CRD with app framework and wait for the pod to be ready
			   * Prepare and deploy Standalone CRD with app framework and wait for the pod to be ready
			   ########## INITIAL VERIFICATION #############
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			   ############### SCALING UP ##################
			   * Scale up Standalone
			   * Wait for Monitoring Console and  Standalone to be ready
			   ########### SCALING UP VERIFICATION #########
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			   ############## SCALING DOWN #################
			   * Scale down Standalone
			   * Wait for Monitoring Console and Standalone to be ready
			   ########### SCALING DOWN VERIFICATION #######
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			*/

			//################## SETUP ####################
			// Upload V1 apps to S3 for Standalone and Monitoring Console
			appVersion := "V1"
			appFileList := testenv.GetAppFileListPhase3(appListV1)
			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Monitoring Console", appVersion))
			s3TestDirMC := "s1appfw-mc-" + testenv.RandomDNSName(4)
			uploadedFiles, err := testenv.UploadFilesToS3(testS3Bucket, s3TestDirMC, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Monitoring Console", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			testenvInstance.Log.Info(fmt.Sprintf("Upload %s apps to S3 for Standalone", appVersion))
			s3TestDir = "s1appfw-" + testenv.RandomDNSName(4)
			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory for Standalone", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework Spec for Monitoring Console
			volumeNameMC := "appframework-test-volume-mc-" + testenv.RandomDNSName(3)
			volumeSpecMC := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeNameMC, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}
			appSourceDefaultSpecMC := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeNameMC,
				Scope:   enterpriseApi.ScopeLocal,
			}
			appSourceNameMC := "appframework-mc-" + testenv.RandomDNSName(3)
			appSourceSpecMC := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceNameMC, s3TestDirMC, appSourceDefaultSpecMC)}
			appFrameworkSpecMC := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpecMC,
				AppsRepoPollInterval: 60,
				VolList:              volumeSpecMC,
				AppSources:           appSourceSpecMC,
			}
			mcSpec := enterpriseApi.MonitoringConsoleSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "IfNotPresent",
					},
					Volumes: []corev1.Volume{},
				},
				AppFrameworkConfig: appFrameworkSpecMC,
			}

			// Deploy Monitoring Console
			testenvInstance.Log.Info("Deploy Monitoring Console")
			mcName := deployment.GetName()
			mc, err := deployment.DeployMonitoringConsoleWithGivenSpec(testenvInstance.GetName(), mcName, mcSpec)
			Expect(err).To(Succeed(), "Unable to deploy Monitoring Console")

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			// Upload apps to S3 for Standalone
			s3TestDir := "s1appfw-" + testenv.RandomDNSName(4)
			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), "Unable to upload apps to S3 test directory")
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework Spec for Standalone
			volumeName := "appframework-test-volume-" + testenv.RandomDNSName(3)
			volumeSpec := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeName, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}
			appSourceDefaultSpec := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeName,
				Scope:   enterpriseApi.ScopeLocal,
			}
			appSourceName := "appframework" + testenv.RandomDNSName(3)
			appSourceSpec := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceName, s3TestDir, appSourceDefaultSpec)}
			appFrameworkSpec := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpec,
				AppsRepoPollInterval: 60,
				VolList:              volumeSpec,
				AppSources:           appSourceSpec,
			}
			spec := enterpriseApi.StandaloneSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "Always",
					},
					Volumes: []corev1.Volume{},
					MonitoringConsoleRef: corev1.ObjectReference{
						Name: mcName,
					},
				},
				AppFrameworkConfig: appFrameworkSpec,
			}

			// Deploy Standalone
			testenvInstance.Log.Info("Deploy Standalone")
			standalone, err := deployment.DeployStandaloneWithGivenSpec(deployment.GetName(), spec)
			Expect(err).To(Succeed(), "Unable to deploy Standalone instance with App framework")

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			//########## INITIAL VERIFICATION #############
			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Download State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify App Copy State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			opLocalAppPathStandalone := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), standalone.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			opPod := testenv.GetOperatorPodName(testenvInstance.GetName())
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify Apps Deleted on Operator Pod for Monitoring Console
			opLocalAppPathMonitoringConsole := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), mc.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathMonitoringConsole)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify App Install State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify V1 apps are deleted on Standalone Pod
			downloadLocationStandalonePod := "/init-apps/" + appSourceName
			standalonePodName := fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify V1 apps are deleted on Monitoring Console Pod
			downloadLocationMCPod := "/init-apps/" + appSourceNameMC
			mcPodName := fmt.Sprintf(testenv.MonitoringConsolePod, mcName, 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Monitoring Console pod %s", appVersion, mcPodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{mcPodName}, appFileList, downloadLocationMCPod)

			// Verify apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on Standalone and Monitoring Console", appVersion))
			podNames := []string{standalonePodName, mcPodName}
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, true)

			// Verify apps are installed
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are installed on Standalone and Monitoring Console", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, "enabled", false, false)

			//############### SCALING UP ##################
			// Scale up Standalone instance
			testenvInstance.Log.Info("Scale up Standalone")
			scaledReplicaCount := 2
			standalone = &enterpriseApi.Standalone{}
			err = deployment.GetInstance(deployment.GetName(), standalone)
			Expect(err).To(Succeed(), "Failed to get instance of Standalone")

			standalone.Spec.Replicas = int32(scaledReplicaCount)

			err = deployment.UpdateCR(standalone)
			Expect(err).To(Succeed(), "Failed to scale up Standalone")

			// Ensure Standalone is scaling up
			testenv.VerifyStandalonePhase(deployment, testenvInstance, deployment.GetName(), splcommon.PhaseScalingUp)

			// Wait for Standalone to be in READY status
			testenv.VerifyStandalonePhase(deployment, testenvInstance, deployment.GetName(), splcommon.PhaseReady)

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			podNames = append(podNames, fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 1))

			//########### SCALING UP VERIFICATION #########

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Download State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify App Copy State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify Apps Deleted on Operator Pod for Monitoring Console
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathMonitoringConsole)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify App Install State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify apps are deleted on Standalone Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName, fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 1)}, appFileList, downloadLocationStandalonePod)

			// Verify apps are deleted on Monitoring Console Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Monitoring Console pod %s", appVersion, mcPodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{mcPodName}, appFileList, downloadLocationMCPod)

			// Verify apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on all pods after scaling up", appVersion))
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, false)

			// Verify apps are installed
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are installed on all pods after scaling up", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, "enabled", false, false)

			//############## SCALING DOWN #################
			// Scale down Standalone instance
			testenvInstance.Log.Info("Scale down Standalone")
			scaledReplicaCount = 1
			standalone = &enterpriseApi.Standalone{}
			err = deployment.GetInstance(deployment.GetName(), standalone)
			Expect(err).To(Succeed(), "Failed to get instance of Standalone after scaling down")

			standalone.Spec.Replicas = int32(scaledReplicaCount)
			err = deployment.UpdateCR(standalone)
			Expect(err).To(Succeed(), "Failed to scale down Standalone")

			// Ensure Standalone is scaling down
			testenv.VerifyStandalonePhase(deployment, testenvInstance, deployment.GetName(), splcommon.PhaseScalingDown)

			// Wait for Standalone to be in READY status
			testenv.VerifyStandalonePhase(deployment, testenvInstance, deployment.GetName(), splcommon.PhaseReady)

			// Verify Monitoring Console is Ready and stays in ready state
			testenv.VerifyMonitoringConsoleReady(deployment, deployment.GetName(), mc, testenvInstance)

			podNames = []string{standalonePodName, mcPodName}

			//########### SCALING DOWN VERIFICATION #######
			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Download State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify App Copy State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify Apps Deleted on Operator Pod for Monitoring Console
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathMonitoringConsole)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify App Install State on Monitoring Console CR
			testenv.VerifyAppListPhaseMonitoringConsole(deployment, testenvInstance, mcName, appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify apps are deleted on Standalone Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify apps are deleted on Monitoring Console Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Monitoring Console pod %s", appVersion, mcPodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{mcPodName}, appFileList, downloadLocationMCPod)

			// Verify apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on all Pods after scaling down ", appVersion))
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, false)

			// Verify apps are installed
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are still installed on the pods after scaling down", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), podNames, appListV1, true, "enabled", false, false)

		})
	})

	// ES App Installation not supported at the time. Will be added back at a later time.
	XContext("Standalone deployment (S1) with App Framework", func() {
		It("s1, integration, appframework: can deploy a Standalone and have ES app installed", func() {

			/* Test Steps
			   ################## SETUP ####################
			   * Upload ES app to S3
			   * Create App Source for Standalone
			   * Prepare and deploy Standalone and wait for the pod to be ready
			   ################## VERIFICATION #############
			   * Verify ES app is installed on Standalone
			*/

			//################## SETUP ####################

			// Download ES App from S3
			testenvInstance.Log.Info("Download ES app from S3")
			esApp := []string{"SplunkEnterpriseSecuritySuite"}
			appFileList := testenv.GetAppFileListPhase3(esApp)
			err := testenv.DownloadFilesFromS3(testDataS3Bucket, s3AppDirV1, downloadDirV1, appFileList)
			Expect(err).To(Succeed(), "Unable to download ES app")

			// Create local directory for file download
			s3TestDir = "s1appfw-" + testenv.RandomDNSName(4)

			// Upload ES app to S3
			testenvInstance.Log.Info("Upload ES app on S3")
			uploadedFiles, err := testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), "Unable to upload ES app to S3 test directory")
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework Spec
			volumeName := "appframework-test-volume-" + testenv.RandomDNSName(3)
			volumeSpec := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeName, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}

			appSourceDefaultSpec := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeName,
				Scope:   enterpriseApi.ScopeLocal,
			}
			appSourceName := "appframework-" + testenv.RandomDNSName(3)
			appSourceSpec := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceName, s3TestDir, appSourceDefaultSpec)}
			appFrameworkSpec := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpec,
				AppsRepoPollInterval: 60,
				VolList:              volumeSpec,
				AppSources:           appSourceSpec,
			}
			spec := enterpriseApi.StandaloneSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "Always",
					},
					Volumes: []corev1.Volume{},
				},
				AppFrameworkConfig: appFrameworkSpec,
			}

			// Deploy Standalone
			testenvInstance.Log.Info("Deploy Standalone")
			standalone, err := deployment.DeployStandaloneWithGivenSpec(deployment.GetName(), spec)
			Expect(err).To(Succeed(), "Unable to deploy Standalone with App framework")

			// Verify App Downlaod State on CR
			// testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify Apps download on Operator Pod
			// kind := standalone.Kind
			// opLocalAppPathStandalone := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			// opPod := testenv.GetOperatorPodName(testenvInstance.GetName())
			appVersion := "V1"
			// testenvInstance.Log.Info("Verify Apps are downloaded on Splunk Operator container for apps", "version", appVersion)
			// testenv.VerifyAppsDownloadedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Ensure Standalone goes to Ready phase
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			//################## VERIFICATION #############
			// Verify ES app is downloaded
			testenvInstance.Log.Info("Verify ES app is downloaded on Standalone")
			initContDownloadLocation := "/init-apps/" + appSourceName
			podName := fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 0)
			testenvInstance.Log.Info("Verify Apps are downloaded on Pod", "version", appVersion, "Pod Name", podName)
			testenv.VerifyAppsDownloadedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{podName}, appFileList, initContDownloadLocation)

			// Verify ES app is installed
			testenvInstance.Log.Info("Verify ES app is installed on Standalone")
			standalonePod := []string{fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 0)}
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), standalonePod, esApp, false, "enabled", false, false)
		})
	})

	Context("Standalone deployment (S1) with App Framework", func() {
		It("integration, s1, appframework: can deploy a Standalone instance with App Framework enabled and install around 350MB of apps at once", func() {

			/* Test Steps
			   ################## SETUP ####################
			   * Create app source for Standalone
			   * Add more apps than usual on S3 for this test
			   * Prepare and deploy Standalone with app framework and wait for the pod to be ready
			   ############### VERIFICATION ################
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled and version by running splunk cmd
			*/

			//################## SETUP ####################

			// Creating a bigger list of apps to be installed than the default one
			appList := append(appListV1, testenv.RestartNeededApps...)
			appFileList := testenv.GetAppFileListPhase3(appList)
			appVersion := "V1"

			// Download apps from S3
			testenvInstance.Log.Info("Download bigger amount of apps from S3 for this test")
			err := testenv.DownloadFilesFromS3(testDataS3Bucket, s3AppDirV1, downloadDirV1, testenv.GetAppFileListPhase3(appList))
			Expect(err).To(Succeed(), "Unable to download apps files")

			// Upload apps to S3
			testenvInstance.Log.Info("Upload bigger amount of apps to S3 for this test")
			s3TestDir = "s1appfw-" + testenv.RandomDNSName(4)
			uploadedFiles, err := testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), "Unable to upload apps to S3 test directory")
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework Spec
			volumeName := "appframework-test-volume-" + testenv.RandomDNSName(3)
			volumeSpec := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeName, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}

			appSourceDefaultSpec := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeName,
				Scope:   enterpriseApi.ScopeLocal,
			}

			appSourceName := "appframework" + testenv.RandomDNSName(3)
			appSourceSpec := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceName, s3TestDir, appSourceDefaultSpec)}

			appFrameworkSpec := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpec,
				AppsRepoPollInterval: 60,
				VolList:              volumeSpec,
				AppSources:           appSourceSpec,
			}

			spec := enterpriseApi.StandaloneSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "Always",
					},
					Volumes: []corev1.Volume{},
				},
				AppFrameworkConfig: appFrameworkSpec,
			}

			// Deploy Standalone
			testenvInstance.Log.Info("Deploy Standalone")
			standalone, err := deployment.DeployStandaloneWithGivenSpec(deployment.GetName(), spec)
			Expect(err).To(Succeed(), "Unable to deploy Standalone instance")

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			//############### VERIFICATION ################

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			opLocalAppPathStandalone := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), standalone.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			opPod := testenv.GetOperatorPodName(testenvInstance.GetName())
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify V1 apps are deleted on Standalone Pod
			downloadLocationStandalonePod := "/init-apps/" + appSourceName
			standalonePodName := fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify apps are copied to correct location
			testenvInstance.Log.Info("Verify apps are copied to correct location on Standalone")
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appList, true, false)

			// Verify apps are installed
			testenvInstance.Log.Info("Verify apps are installed on Standalone")
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appList, true, "enabled", false, false)
		})
	})

	Context("Standalone deployment (S1) with App Framework", func() {
		It("s1, smoke, appframework: can deploy a standalone instance with App Framework enabled for manual poll", func() {

			/* Test Steps
			   ################## SETUP ####################
			   * Create app source for Standalone
			   * Prepare and deploy Standalone with app framework(MANUAL POLL) and wait for the pod to be ready
			   ############### VERIFICATION ################
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled and version by running splunk cmd
			     ############ UPGRADE V2 APPS ###########
			   * Upload V2 apps to S3 App Source
			   ############ VERIFICATION APPS ARE NOT UPDATED BEFORE ENABLING MANUAL POLL ############
			   * Verify Apps are not updated
			   ############ ENABLE MANUAL POLL ############
			   * Verify Manual Poll disabled after the check
			   ############ V2 APP VERIFICATION FOR STANDALONE AND MONITORING CONSOLE  ###########
			   * Verify Apps Downloaded in App Deployment Info
			   * Verify Apps Copied in App Deployment Info
			   * Verify App Package is deleted from Operator Pod
			   * Verify Apps Installed in App Deployment Info
			   * Verify App Package is deleted from Splunk Pod
			   * Verify App Directory in under splunk path
			   * Verify App enabled  and version by running splunk cmd
			*/

			//################## SETUP ####################

			// Upload V1 apps to S3
			appVersion := "V1"
			s3TestDir := "s1appfw-mc-" + testenv.RandomDNSName(4)
			appFileList := testenv.GetAppFileListPhase3(appListV1)
			uploadedFiles, err := testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV1)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Create App framework Spec
			volumeName := "appframework-test-volume-" + testenv.RandomDNSName(3)
			volumeSpec := []enterpriseApi.VolumeSpec{testenv.GenerateIndexVolumeSpec(volumeName, testenv.GetS3Endpoint(), testenvInstance.GetIndexSecretName(), "aws", "s3")}

			// AppSourceDefaultSpec: Remote Storage volume name and Scope of App deployment
			appSourceDefaultSpec := enterpriseApi.AppSourceDefaultSpec{
				VolName: volumeName,
				Scope:   enterpriseApi.ScopeLocal,
			}

			// appSourceSpec: App source name, location and volume name and scope from appSourceDefaultSpec
			appSourceName := "appframework" + testenv.RandomDNSName(3)
			appSourceSpec := []enterpriseApi.AppSourceSpec{testenv.GenerateAppSourceSpec(appSourceName, s3TestDir, appSourceDefaultSpec)}

			// appFrameworkSpec: AppSource settings, Poll Interval, volumes, appSources on volumes
			appFrameworkSpec := enterpriseApi.AppFrameworkSpec{
				Defaults:             appSourceDefaultSpec,
				AppsRepoPollInterval: 0,
				VolList:              volumeSpec,
				AppSources:           appSourceSpec,
			}

			spec := enterpriseApi.StandaloneSpec{
				CommonSplunkSpec: enterpriseApi.CommonSplunkSpec{
					Spec: splcommon.Spec{
						ImagePullPolicy: "Always",
					},
					Volumes: []corev1.Volume{},
				},
				AppFrameworkConfig: appFrameworkSpec,
			}

			// Create Standalone Deployment with App Framework
			standalone, err := deployment.DeployStandaloneWithGivenSpec(deployment.GetName(), spec)
			Expect(err).To(Succeed(), "Unable to deploy standalone instance with App framework")

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			//############### VERIFICATION ################

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			opLocalAppPathStandalone := filepath.Join(splcommon.AppDownloadVolume, "downloadedApps", testenvInstance.GetName(), standalone.Kind, deployment.GetName(), enterpriseApi.ScopeLocal, appSourceName)
			opPod := testenv.GetOperatorPodName(testenvInstance.GetName())
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify V1 apps are deleted on Standalone Pod
			downloadLocationStandalonePod := "/init-apps/" + appSourceName
			standalonePodName := fmt.Sprintf(testenv.StandalonePod, deployment.GetName(), 0)
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			// Verify Apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on standalone(/etc/apps/)", appVersion))
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appListV1, true, true)

			//Verify Apps are installed
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are installed Locally on CM and Deployer", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appListV1, true, "enabled", false, false)

			//############### UPGRADE APPS ################

			//Delete apps on S3 for new Apps
			testenv.DeleteFilesOnS3(testS3Bucket, uploadedApps)
			uploadedApps = nil

			//Upload new Versioned Apps to S3
			appVersion = "V2"
			appFileList = testenv.GetAppFileListPhase3(appListV2)
			uploadedFiles, err = testenv.UploadFilesToS3(testS3Bucket, s3TestDir, appFileList, downloadDirV2)
			Expect(err).To(Succeed(), fmt.Sprintf("Unable to upload %s apps to S3 test directory", appVersion))
			uploadedApps = append(uploadedApps, uploadedFiles...)

			// Wait for the poll period for the apps to be downloaded
			time.Sleep(2 * time.Minute)

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			// ############ VERIFICATION APPS ARE NOT UPDATED BEFORE ENABLING MANUAL POLL ############

			//Verify Apps are not updated
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are not auto installed on standalone", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appListV1, true, "enabled", false, false)

			// ############ ENABLE MANUAL POLL ############

			testenvInstance.Log.Info("Get config map for triggering manual update")
			config, err := testenv.GetAppframeworkManualUpdateConfigMap(deployment, testenvInstance.GetName())
			Expect(err).To(Succeed(), "Unable to get config map for manual poll")
			testenvInstance.Log.Info("Config map data for", "Standalone", config.Data["Standalone"])

			testenvInstance.Log.Info("Modify config map to trigger manual update")
			config.Data["Standalone"] = strings.Replace(config.Data["Standalone"], "off", "on", 1)
			err = deployment.UpdateCR(config)
			Expect(err).To(Succeed(), "Unable to update config map")

			// Wait for Standalone to be in READY status
			testenv.StandaloneReady(deployment, deployment.GetName(), standalone, testenvInstance)

			//Verify config map set back to off after poll trigger
			testenvInstance.Log.Info(fmt.Sprintf("Verify config map set back to off after poll trigger for %s app", appVersion))
			config, _ = testenv.GetAppframeworkManualUpdateConfigMap(deployment, testenvInstance.GetName())
			Expect(strings.Contains(config.Data["Standalone"], "status: off")).To(Equal(true), "Config map update not complete")

			//############### VERIFICATION FOR UPGRADE ################

			// Verify App Download State on Standlaone CR
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseDownload, appFileList)

			// Verify App Copy State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhasePodCopy, appFileList)

			// Verify Apps Deleted on Operator Pod for Standalone
			testenvInstance.Log.Info(fmt.Sprintf("Verify Apps are deleted on Splunk Operator for version %s", appVersion))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{opPod}, appFileList, opLocalAppPathStandalone)

			// Verify App Install State on Standalone CR
			appFileList = testenv.GetAppFileListPhase3(appListV1)
			testenv.VerifyAppListPhaseStandalone(deployment, testenvInstance, deployment.GetName(), appSourceName, enterpriseApi.PhaseInstall, appFileList)

			// Verify V1 apps are deleted on Standalone Pod
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are deleted on Standalone pod %s", appVersion, standalonePodName))
			testenv.VerifyAppsPackageDeletedOnContainer(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appFileList, downloadLocationStandalonePod)

			//Verify Apps are copied to location
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are copied to correct location on standalone(/etc/apps/)", appVersion))
			testenv.VerifyAppsCopied(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appListV2, true, true)

			//Verify Apps are updated
			testenvInstance.Log.Info(fmt.Sprintf("Verify %s apps are installed Locally on standalone", appVersion))
			testenv.VerifyAppInstalled(deployment, testenvInstance, testenvInstance.GetName(), []string{standalonePodName}, appListV2, true, "enabled", true, false)
		})
	})
})
