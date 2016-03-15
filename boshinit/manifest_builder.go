package boshinit

import "github.com/pivotal-cf-experimental/bosh-bootloader/ssl"

type ManifestBuilder struct {
	logger                       logger
	sslKeyPairGenerator          sslKeyPairGenerator
	uuidGenerator                UUIDGenerator
	cloudProviderManifestBuilder cloudProviderManifestBuilder
	jobsManifestBuilder          jobsManifestBuilder
}

type ManifestProperties struct {
	DirectorUsername string
	DirectorPassword string
	SubnetID         string
	AvailabilityZone string
	ElasticIP        string
	AccessKeyID      string
	SecretAccessKey  string
	DefaultKeyName   string
	Region           string
	SecurityGroup    string
	SSLKeyPair       ssl.KeyPair
}

type logger interface {
	Step(message string)
}

type sslKeyPairGenerator interface {
	Generate(commonName string) (ssl.KeyPair, error)
}

type UUIDGenerator interface {
	Generate() (string, error)
}

type cloudProviderManifestBuilder interface {
	Build(ManifestProperties) (CloudProvider, error)
}

type jobsManifestBuilder interface {
	Build(ManifestProperties) ([]Job, error)
}

func NewManifestBuilder(logger logger, sslKeyPairGenerator sslKeyPairGenerator, uuidGenerator UUIDGenerator, cloudProviderManifestBuilder cloudProviderManifestBuilder, jobsManifestBuilder jobsManifestBuilder) ManifestBuilder {
	return ManifestBuilder{
		logger:                       logger,
		sslKeyPairGenerator:          sslKeyPairGenerator,
		uuidGenerator:                uuidGenerator,
		cloudProviderManifestBuilder: cloudProviderManifestBuilder,
		jobsManifestBuilder:          jobsManifestBuilder,
	}
}

func (m ManifestBuilder) Build(manifestProperties ManifestProperties) (Manifest, ManifestProperties, error) {
	m.logger.Step("generating bosh-init manifest")

	releaseManifestBuilder := NewReleaseManifestBuilder()
	resourcePoolsManifestBuilder := NewResourcePoolsManifestBuilder()
	diskPoolsManifestBuilder := NewDiskPoolsManifestBuilder()
	networksManifestBuilder := NewNetworksManifestBuilder()

	if !manifestProperties.SSLKeyPair.IsValidForIP(manifestProperties.ElasticIP) {
		keyPair, err := m.sslKeyPairGenerator.Generate(manifestProperties.ElasticIP)
		if err != nil {
			return Manifest{}, ManifestProperties{}, err
		}

		manifestProperties.SSLKeyPair = keyPair
	}

	cloudProvider, err := m.cloudProviderManifestBuilder.Build(manifestProperties)
	if err != nil {
		return Manifest{}, ManifestProperties{}, err
	}

	jobs, err := m.jobsManifestBuilder.Build(manifestProperties)
	if err != nil {
		return Manifest{}, ManifestProperties{}, err
	}

	return Manifest{
		Name:          "bosh",
		Releases:      releaseManifestBuilder.Build(),
		ResourcePools: resourcePoolsManifestBuilder.Build(manifestProperties),
		DiskPools:     diskPoolsManifestBuilder.Build(),
		Networks:      networksManifestBuilder.Build(manifestProperties),
		Jobs:          jobs,
		CloudProvider: cloudProvider,
	}, manifestProperties, nil
}