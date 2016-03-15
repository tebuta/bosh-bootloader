package boshinit_test

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/pivotal-cf-experimental/bosh-bootloader/boshinit"
	"github.com/pivotal-cf-experimental/bosh-bootloader/fakes"
	"github.com/pivotal-cf-experimental/bosh-bootloader/ssl"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("ManifestBuilder", func() {
	var (
		logger                       *fakes.Logger
		sslKeyPairGenerator          *fakes.SSLKeyPairGenerator
		uuidGenerator                *fakes.UUIDGenerator
		manifestBuilder              boshinit.ManifestBuilder
		manifestProperties           boshinit.ManifestProperties
		cloudProviderManifestBuilder boshinit.CloudProviderManifestBuilder
		jobsManifestBuilder          boshinit.JobsManifestBuilder
	)

	BeforeEach(func() {
		logger = &fakes.Logger{}
		sslKeyPairGenerator = &fakes.SSLKeyPairGenerator{}
		uuidGenerator = &fakes.UUIDGenerator{}
		cloudProviderManifestBuilder = boshinit.NewCloudProviderManifestBuilder(uuidGenerator)
		jobsManifestBuilder = boshinit.NewJobsManifestBuilder(uuidGenerator)

		manifestBuilder = boshinit.NewManifestBuilder(logger, sslKeyPairGenerator, uuidGenerator, cloudProviderManifestBuilder, jobsManifestBuilder)
		manifestProperties = boshinit.ManifestProperties{
			DirectorUsername: "bosh-username",
			DirectorPassword: "bosh-password",
			SubnetID:         "subnet-12345",
			AvailabilityZone: "some-az",
			ElasticIP:        "52.0.112.12",
			AccessKeyID:      "some-access-key-id",
			SecretAccessKey:  "some-secret-access-key",
			DefaultKeyName:   "some-key-name",
			Region:           "some-region",
			SecurityGroup:    "some-security-group",
		}

		uuidGenerator.GenerateCall.Returns = []fakes.GenerateReturn{
			{String: "randomly-generated-mbus-password"},
			{String: "randomly-generated-nats-password"},
			{String: "randomly-generated-redis-password"},
			{String: "randomly-generated-postgres-password"},
			{String: "randomly-generated-registry-password"},
			{String: "randomly-generated-blobstore-director-password"},
			{String: "randomly-generated-blobstore-agent-password"},
			{String: "randomly-generated-hm-password"},
		}
	})

	Describe("Build", func() {
		It("builds the bosh-init manifest and updates the manifest properties", func() {
			sslKeyPairGenerator.GenerateCall.Returns.KeyPair = ssl.KeyPair{
				Certificate: []byte(certificate),
				PrivateKey:  []byte(privateKey),
			}

			manifest, manifestProperties, err := manifestBuilder.Build(manifestProperties)
			Expect(err).NotTo(HaveOccurred())

			expectedAWSProperties := boshinit.AWSProperties{
				AccessKeyId:           "some-access-key-id",
				SecretAccessKey:       "some-secret-access-key",
				DefaultKeyName:        "some-key-name",
				DefaultSecurityGroups: []string{"some-security-group"},
				Region:                "some-region",
			}

			Expect(manifest.Name).To(Equal("bosh"))
			Expect(manifest.Releases[0].Name).To(Equal("bosh"))
			Expect(manifest.ResourcePools[0].CloudProperties.AvailabilityZone).To(Equal("some-az"))
			Expect(manifest.DiskPools[0].Name).To(Equal("disks"))
			Expect(manifest.Networks[0].Subnets[0].CloudProperties.Subnet).To(Equal("subnet-12345"))
			Expect(manifest.Jobs[0].Networks[1].StaticIPs[0]).To(Equal("52.0.112.12"))
			Expect(manifest.Jobs[0].Properties.AWS).To(Equal(expectedAWSProperties))
			Expect(manifest.Jobs[0].Properties.Director.SSL).To(Equal(boshinit.SSLProperties{
				Cert: certificate,
				Key:  privateKey,
			}))
			Expect(manifest.CloudProvider.Properties.AWS).To(Equal(expectedAWSProperties))
			Expect(manifest.CloudProvider.SSHTunnel.Host).To(Equal("52.0.112.12"))
			Expect(manifest.CloudProvider.MBus).To(Equal("https://mbus:randomly-generated-mbus-password@52.0.112.12:6868"))

			Expect(sslKeyPairGenerator.GenerateCall.Receives.Name).To(Equal("52.0.112.12"))
			Expect(sslKeyPairGenerator.GenerateCall.CallCount).To(Equal(1))

			Expect(manifestProperties).To(Equal(
				boshinit.ManifestProperties{
					DirectorUsername: "bosh-username",
					DirectorPassword: "bosh-password",
					SubnetID:         "subnet-12345",
					AvailabilityZone: "some-az",
					ElasticIP:        "52.0.112.12",
					AccessKeyID:      "some-access-key-id",
					SecretAccessKey:  "some-secret-access-key",
					DefaultKeyName:   "some-key-name",
					Region:           "some-region",
					SecurityGroup:    "some-security-group",
					SSLKeyPair: ssl.KeyPair{
						Certificate: []byte(certificate),
						PrivateKey:  []byte(privateKey),
					},
				},
			))
		})

		It("does not generate an ssl keypair if it exists", func() {
			manifestProperties.SSLKeyPair = ssl.KeyPair{
				Certificate: []byte(certificate),
				PrivateKey:  []byte(privateKey),
			}

			_, _, err := manifestBuilder.Build(manifestProperties)
			Expect(err).NotTo(HaveOccurred())
			Expect(sslKeyPairGenerator.GenerateCall.CallCount).To(Equal(0))
		})

		It("logs that the bosh-init manifest is being generated", func() {
			_, _, err := manifestBuilder.Build(manifestProperties)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.StepCall.Receives.Message).To(Equal("generating bosh-init manifest"))
		})

		Context("failure cases", func() {
			It("returns an error when the ssl key pair cannot be generated", func() {
				sslKeyPairGenerator.GenerateCall.Returns.Error = errors.New("failed to generate key pair")

				_, _, err := manifestBuilder.Build(manifestProperties)
				Expect(err).To(MatchError("failed to generate key pair"))
			})

			Context("failing cloud provider manifest builder", func() {
				BeforeEach(func() {
					fakeCloudProviderManifestBuilder := &fakes.CloudProviderManifestBuilder{}
					fakeCloudProviderManifestBuilder.BuildCall.Returns.Error = fmt.Errorf("something bad happened")
					manifestBuilder = boshinit.NewManifestBuilder(logger, sslKeyPairGenerator, uuidGenerator, fakeCloudProviderManifestBuilder, jobsManifestBuilder)
					manifestProperties = boshinit.ManifestProperties{
						DirectorUsername: "bosh-username",
						DirectorPassword: "bosh-password",
						SubnetID:         "subnet-12345",
						AvailabilityZone: "some-az",
						ElasticIP:        "52.0.112.12",
						AccessKeyID:      "some-access-key-id",
						SecretAccessKey:  "some-secret-access-key",
						DefaultKeyName:   "some-key-name",
						Region:           "some-region",
						SecurityGroup:    "some-security-group",
					}
				})
				It("returns an error when it cannot build the cloud provider manifest", func() {
					_, _, err := manifestBuilder.Build(manifestProperties)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("failing jobs manifest builder", func() {
				BeforeEach(func() {
					fakeJobsManifestBuilder := &fakes.JobsManifestBuilder{}
					fakeJobsManifestBuilder.BuildCall.Returns.Error = fmt.Errorf("something bad happened")
					manifestBuilder = boshinit.NewManifestBuilder(logger, sslKeyPairGenerator, uuidGenerator, cloudProviderManifestBuilder, fakeJobsManifestBuilder)
					manifestProperties = boshinit.ManifestProperties{
						DirectorUsername: "bosh-username",
						DirectorPassword: "bosh-password",
						SubnetID:         "subnet-12345",
						AvailabilityZone: "some-az",
						ElasticIP:        "52.0.112.12",
						AccessKeyID:      "some-access-key-id",
						SecretAccessKey:  "some-secret-access-key",
						DefaultKeyName:   "some-key-name",
						Region:           "some-region",
						SecurityGroup:    "some-security-group",
					}
				})
				It("returns an error when it cannot build the job manifest", func() {
					_, _, err := manifestBuilder.Build(manifestProperties)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Describe("template marshaling", func() {
		It("can be marshaled to YML", func() {
			sslKeyPairGenerator.GenerateCall.Returns.KeyPair = ssl.KeyPair{
				Certificate: []byte(certificate),
				PrivateKey:  []byte(privateKey),
			}

			manifest, _, err := manifestBuilder.Build(manifestProperties)
			Expect(err).NotTo(HaveOccurred())

			buf, err := ioutil.ReadFile("fixtures/manifest.yml")
			Expect(err).NotTo(HaveOccurred())

			output, err := candiedyaml.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(MatchYAML(string(buf)))
		})
	})
})