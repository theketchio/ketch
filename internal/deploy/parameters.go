package deploy

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/errors"
	"github.com/shipa-corp/ketch/internal/utils"
)

const (
	flagImage          = "image"
	flagKetchYaml      = "ketch-yaml"
	flagProcFile       = "procfile"
	flagStrict         = "strict"
	flagSteps          = "steps"
	flagStepInterval   = "step-interval"
	flagWait           = "wait"
	flagTimeout        = "timeout"
	flagIncludeDirs    = "include-dirs"
	flagPlatform       = "platform"
	flagDescription    = "description"
	flagEnvironment    = "env"
	flagPool           = "pool"
	flagRegistrySecret = "registry-secret"
	flagBuilder        = "builder"
	flagBuildPacks     = "build-packs"

	flagImageShort       = "i"
	flagPlatformShort    = "P"
	flagDescriptionShort = "d"
	flagEnvironmentShort = "e"
	flagPoolShort        = "o"

	defaultYamlFile = "ketch.yaml"
)

// Options receive values set in flags.  They are processed into a ChangeSet
// which describes the values that have been explicitly set by the end user. In
// this way we know if we will need to update an existing app CRD.
type Options struct {
	AppName                 string
	Image                   string
	KetchYamlFileName       string
	ProcfileFileName        string
	StrictKetchYamlDecoding bool
	Steps                   int
	StepTimeInterval        string
	Wait                    bool
	Timeout                 string
	AppSourcePath           string
	SubPaths                []string

	Pool                 string
	Description          string
	Envs                 []string
	DockerRegistrySecret string
	// this goes bye bye
	Platform   string
	Builder    string
	BuildPacks []string
}

type ChangeSet struct {
	appName              string
	yamlStrictDecoding   bool
	sourcePath           *string
	sourceSubPaths       *[]string
	image                *string
	ketchYamlFileName    *string
	procfileFileName     *string
	steps                *int
	stepTimeInterval     *string
	wait                 *bool
	timeout              *string
	subPaths             *[]string
	platform             *string
	description          *string
	envs                 *[]string
	pool                 *string
	dockerRegistrySecret *string
	builder              *string
	buildPacks           *[]string
}

func (o Options) GetChangeSet(flags *pflag.FlagSet) *ChangeSet {
	var cs ChangeSet
	cs.appName = o.AppName
	cs.yamlStrictDecoding = o.StrictKetchYamlDecoding

	if o.AppSourcePath != "" {
		cs.sourcePath = &o.AppSourcePath
	}
	m := map[string]func(c *ChangeSet){
		flagImage: func(c *ChangeSet) {
			c.image = &o.Image
		},
		flagKetchYaml: func(c *ChangeSet) {
			c.ketchYamlFileName = &o.KetchYamlFileName
		},
		flagProcFile: func(c *ChangeSet) {
			c.procfileFileName = &o.ProcfileFileName
		},
		flagSteps: func(c *ChangeSet) {
			c.steps = &o.Steps
		},
		flagStepInterval: func(c *ChangeSet) {
			c.stepTimeInterval = &o.StepTimeInterval
		},
		flagWait: func(c *ChangeSet) {
			c.wait = &o.Wait
		},
		flagTimeout: func(c *ChangeSet) {
			c.timeout = &o.Timeout
		},
		flagIncludeDirs: func(c *ChangeSet) {
			c.subPaths = &o.SubPaths
		},
		flagPlatform: func(c *ChangeSet) {
			c.platform = &o.Platform
		},
		flagDescription: func(c *ChangeSet) {
			c.description = &o.Description
		},
		flagEnvironment: func(c *ChangeSet) {
			c.envs = &o.Envs
		},
		flagPool: func(c *ChangeSet) {
			c.pool = &o.Pool
		},
		flagRegistrySecret: func(c *ChangeSet) {
			c.dockerRegistrySecret = &o.DockerRegistrySecret
		},
		flagBuilder: func(c *ChangeSet) {
			c.builder = &o.Builder
		},
		flagBuildPacks: func(c *ChangeSet) {
			c.buildPacks = &o.BuildPacks
		},
	}
	for k, f := range m {
		if flags.Changed(k) {
			f(&cs)
		}
	}
	return &cs
}

func (c *ChangeSet) getProcfileName() (string, error) {
	if c.procfileFileName == nil {
		return "", newMissingError(flagProcFile)
	}
	return *c.procfileFileName, nil
}

func (c *ChangeSet) getPlatform(ctx context.Context, client getter) (string, error) {
	if c.platform == nil {
		return "", newMissingError(flagPlatform)
	}
	var p ketchv1.Platform
	err := client.Get(ctx, types.NamespacedName{Name: *c.platform}, &p)
	if apierrors.IsNotFound(err) {
		return "", fmt.Errorf("%w platform %q has not been created", newInvalidError(flagPlatform), *c.platform)
	}
	if err != nil {
		return "", errors.Wrap(err, "could not fetch platform %q", *c.platform)
	}
	return *c.platform, nil
}

func (c *ChangeSet) getDescription() (string, error) {
	if c.description == nil {
		return "", newMissingError(flagDescription)
	}
	return *c.description, nil
}

func (c *ChangeSet) getIncludeDirs() ([]string, error) {
	if c.subPaths == nil {
		return nil, newMissingError(flagIncludeDirs)
	}
	rootDir, err := c.getSourceDirectory()
	if err != nil {
		return nil, err
	}
	paths := *c.subPaths
	for _, p := range paths {
		if err := directoryExists(path.Join(rootDir, p)); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func (c *ChangeSet) getYamlPath() (string, error) {
	if c.ketchYamlFileName == nil {
		return "", newMissingError(flagKetchYaml)
	}
	stat, err := os.Stat(*c.ketchYamlFileName)
	if err != nil {
		return "", newInvalidError(flagKetchYaml)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("%w %s is not a regular file", newInvalidError(flagKetchYaml), *c.ketchYamlFileName)
	}
	return *c.ketchYamlFileName, nil
}

func (c *ChangeSet) getSourceDirectory() (string, error) {
	if c.sourcePath == nil {
		return "", newMissingError("source directory")
	}
	if err := directoryExists(*c.sourcePath); err != nil {
		return "", err
	}
	return *c.sourcePath, nil
}

func (c *ChangeSet) getPool(ctx context.Context, client getter) (string, error) {
	if c.pool == nil {
		return "", newMissingError(flagPool)
	}
	var p ketchv1.Pool
	err := client.Get(ctx, types.NamespacedName{Name: *c.pool}, &p)
	if apierrors.IsNotFound(err) {
		return "", fmt.Errorf("%w pool %q has not been created", newInvalidError(flagPool), *c.pool)
	}
	if err != nil {
		return "", errors.Wrap(err, "could not fetch pool %q", *c.pool)
	}
	return *c.pool, nil
}

func (c *ChangeSet) getImage() (string, error) {
	if c.image == nil {
		return "", fmt.Errorf("%w %s is required", newMissingError(flagImage), flagImage)
	}
	return *c.image, nil
}

func (c *ChangeSet) getSteps() (int, error) {
	if c.steps == nil {
		return 0, newMissingError(flagSteps)
	}
	steps := *c.steps
	if steps < minimumSteps || steps > maximumSteps {
		return 0, fmt.Errorf("%w %s must be between %d and %d",
			newInvalidError(flagSteps), flagSteps, minimumSteps, maximumSteps)
	}
	if maximumSteps%steps != 0 {
		return 0, fmt.Errorf("%w %d must be evenly divisable by %d",
			newInvalidError(flagSteps), maximumSteps, steps)
	}
	return *c.steps, nil
}

func (c *ChangeSet) getStepInterval() (time.Duration, error) {
	if c.stepTimeInterval == nil {
		return 0, newMissingError(flagStepInterval)
	}
	dur, err := time.ParseDuration(*c.stepTimeInterval)
	if err != nil {
		return 0, newInvalidError(flagStepInterval)
	}
	return dur, nil
}

func (c *ChangeSet) getStepWeight() (uint8, error) {
	steps, err := c.getSteps()
	if err != nil {
		return 0, err
	}
	return uint8(steps / maximumSteps), nil
}

func (c *ChangeSet) getEnvironments() ([]ketchv1.Env, error) {
	if c.envs == nil {
		return nil, newMissingError(flagEnvironment)
	}
	envs, err := utils.MakeEnvironments(*c.envs)
	if err != nil {
		return nil, newInvalidError(flagEnvironment)
	}
	return envs, nil
}

func (c *ChangeSet) getWait() (bool, error) {
	if c.wait == nil {
		return false, newMissingError(flagWait)
	}
	return *c.wait, nil
}

func (c *ChangeSet) getTimeout() (time.Duration, error) {
	if c.timeout == nil {
		return 0, newMissingError(flagTimeout)
	}
	d, err := time.ParseDuration(*c.timeout)
	if err != nil {
		return 0, newInvalidError(flagTimeout)
	}
	return d, nil
}

func (c *ChangeSet) getDockerRegistrySecret() (string, error) {
	if c.dockerRegistrySecret == nil {
		return "", newMissingError(flagRegistrySecret)
	}
	return *c.dockerRegistrySecret, nil
}

func (c *ChangeSet) getBuilder() (string, error) {
	if c.builder == nil {
		return "", newMissingError(flagBuilder)
	}
	return *c.builder, nil
}

func (c *ChangeSet) getBuildPacks() ([]string, error) {
	if c.buildPacks == nil {
		return nil, newMissingError(flagBuildPacks)
	}
	return *c.buildPacks, nil
}

func (c *ChangeSet) getKetchYaml() (*ketchv1.KetchYamlData, error) {
	var fileName string
	// try to find yaml file in default location
	sourcePath, err := c.getSourceDirectory()
	if !isMissing(err) && isValid(err) {
		yamlPath := path.Join(sourcePath, defaultYamlFile)
		if stat, err := os.Stat(yamlPath); err == nil && !stat.IsDir() {
			fileName = yamlPath
		}
	}

	// if the yaml path is supplied on the  command line it takes precedence over
	// default yaml file
	yamlPath, err := c.getYamlPath()
	if !isMissing(err) && isValid(err) {
		fileName = yamlPath
	}

	// if no yaml is provided we're done
	if fileName == "" {
		return nil, nil
	}

	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	var decodeOpts []yaml.JSONOpt
	if c.yamlStrictDecoding {
		decodeOpts = append(decodeOpts, yaml.DisallowUnknownFields)
	}
	data := &ketchv1.KetchYamlData{}
	if err = yaml.Unmarshal(content, data, decodeOpts...); err != nil {
		return nil, err
	}
	return data, nil
}
