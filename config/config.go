/*
 * Copyright (c) 2020. Ant Group. All rights reserved.
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package config

import (
	"os"

	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"github.com/containerd/nydus-snapshotter/cmd/containerd-nydus-grpc/pkg/command"
	"github.com/containerd/nydus-snapshotter/pkg/errdefs"
)

// Define a policy how to fork nydusd daemon and attach file system instances to serve.
type DaemonMode string

const (
	// One nydusd, one rafs instance
	DaemonModeMultiple DaemonMode = "multiple"
	// One nydusd serves multiple rafs instances
	DaemonModeShared DaemonMode = "shared"
	// No nydusd daemon is needed to be started. Snapshotter does not start any nydusd
	// and only interacts with containerd with mount slice to pass necessary configuration
	// to container runtime
	DaemonModeNone    DaemonMode = "none"
	DaemonModeInvalid DaemonMode = ""
)

func parseDaemonMode(m string) (DaemonMode, error) {
	switch m {
	case string(DaemonModeMultiple):
		return DaemonModeMultiple, nil
	case string(DaemonModeShared):
		return DaemonModeShared, nil
	case string(DaemonModeNone):
		return DaemonModeNone, nil
	default:
		return DaemonModeInvalid, errdefs.ErrInvalidArgument
	}
}

type DaemonRecoverPolicy int

const (
	RecoverPolicyInvalid DaemonRecoverPolicy = iota
	RecoverPolicyNone
	RecoverPolicyRestart
	RecoverPolicyFailover
)

func (p DaemonRecoverPolicy) String() string {
	switch p {
	case RecoverPolicyNone:
		return "none"
	case RecoverPolicyRestart:
		return "restart"
	case RecoverPolicyFailover:
		return "failover"
	case RecoverPolicyInvalid:
		fallthrough
	default:
		return ""
	}
}

var recoverPolicyParser map[string]DaemonRecoverPolicy

func init() {
	recoverPolicyParser = map[string]DaemonRecoverPolicy{
		RecoverPolicyNone.String():     RecoverPolicyNone,
		RecoverPolicyRestart.String():  RecoverPolicyRestart,
		RecoverPolicyFailover.String(): RecoverPolicyFailover}
}

func ParseRecoverPolicy(p string) (DaemonRecoverPolicy, error) {
	policy, ok := recoverPolicyParser[p]
	if !ok {
		return RecoverPolicyInvalid, errdefs.ErrNotFound
	}

	return policy, nil
}

const (
	FsDriverFusedev string = "fusedev"
	FsDriverFscache string = "fscache"
)

// Configure how to start and recover nydusd daemons
type DaemonConfig struct {
	NydusdPath       string `toml:"nydusd_path"`
	NydusImagePath   string `toml:"nydusimage_path"`
	NydusdConfigPath string `toml:"nydusd_config"`
	RecoverPolicy    string `toml:"recover_policy"`
	FsDriver         string `toml:"fs_driver"`
	ThreadsNumber    int    `toml:"threads_number"`
}

type LoggingConfig struct {
	LogToStdout         bool   `toml:"log_to_stdout"`
	LogLevel            string `toml:"level"`
	LogDir              string `toml:"dir"`
	RotateLogMaxSize    int    `toml:"log_rotation_max_size"`
	RotateLogMaxBackups int    `toml:"log_rotation_max_backups"`
	RotateLogMaxAge     int    `toml:"log_rotation_max_age"`
	RotateLogLocalTime  bool   `toml:"log_rotation_local_time"`
	RotateLogCompress   bool   `toml:"log_rotation_compress"`
}

// Nydus image layers additional process
type ImageConfig struct {
	PublicKeyFile     string `toml:"public_key_file"`
	ValidateSignature bool   `toml:"validate_signature"`
}

// Configure containerd snapshots interfaces and how to process the snapshots
// requests from containerd
type SnapshotConfig struct {
	EnableNydusOverlayFS bool `toml:"enable_nydus_overlayfs"`
	SyncRemove           bool `toml:"sync_remove"`
}

// Configure cache manager that manages the cache files lifecycle
type CacheManagerConfig struct {
	Disable bool `toml:"disable"`
	// Trigger GC gc_period after the specified period.
	// Example format: 24h, 120min
	GCPeriod string `toml:"gc_period"`
	CacheDir string `toml:"cache_dir"`
}

// Configure how nydus-snapshotter receive auth information
type AuthConfig struct {
	// based on kubeconfig or ServiceAccount
	EnableKubeconfigKeychain bool   `toml:"enable_kubeconfig_keychain"`
	KubeconfigPath           string `toml:"kubeconfig_path"`
	// CRI proxy mode
	EnableCRIKeychain   bool   `toml:"enable_cri_keychain"`
	ImageServiceAddress string `toml:"image_service_address"`
}

// Configure remote storage like container registry
type RemoteConfig struct {
	AuthConfig         AuthConfig `toml:"auth"`
	ConvertVpcRegistry bool       `toml:"convert_vpc_registry"`
}

type MetricsConfig struct {
	Address string `toml:"address"`
}

type SnapshotterConfig struct {
	// Configuration format version
	Version int `toml:"version"`
	// Snapshotter's root work directory
	Root                   string `toml:"root"`
	Address                string `toml:"address"`
	EnableSystemController bool   `toml:"enable_system_controller"`
	DaemonMode             string `toml:"daemon_mode"`
	EnableStargz           bool   `toml:"enable_stargz"`
	// Clean up all the resources when snapshotter is closed
	CleanupOnClose bool `toml:"cleanup_on_close"`

	MetricsConfig      MetricsConfig      `toml:"metrics"`
	DaemonConfig       DaemonConfig       `toml:"daemon"`
	SnapshotsConfig    SnapshotConfig     `toml:"snapshot"`
	RemoteConfig       RemoteConfig       `toml:"remote"`
	ImageConfig        ImageConfig        `toml:"image"`
	CacheManagerConfig CacheManagerConfig `toml:"cache_manager"`
	LoggingConfig      LoggingConfig      `toml:"log"`
}

func LoadSnapshotterConfig(path string) (*SnapshotterConfig, error) {
	var config SnapshotterConfig
	// get nydus-snapshotter configuration from specified path of toml file
	if path == "" {
		return nil, errors.New("snapshotter configuration path cannot be empty")
	}
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "load snapshotter configuration file %q", path)
	}
	if err = tree.Unmarshal(&config); err != nil {
		return nil, errors.Wrapf(err, "unmarshal snapshotter configuration file %q", path)
	}
	return &config, nil
}

func MergeConfig(to, from *SnapshotterConfig) error {
	err := mergo.Merge(to, from)
	if err != nil {
		return err
	}

	return nil
}

func ValidateConfig(c *SnapshotterConfig) error {
	if c == nil {
		return errors.Wrapf(errdefs.ErrInvalidArgument, "configuration is none")
	}

	if c.ImageConfig.ValidateSignature {
		if c.ImageConfig.PublicKeyFile == "" {
			return errors.New("need to specify publicKey file for signature validation")
		} else if _, err := os.Stat(c.ImageConfig.PublicKeyFile); err != nil {
			return errors.Wrapf(err, "failed to find publicKey file %q", c.ImageConfig.PublicKeyFile)
		}
	}

	daemonMode, err := parseDaemonMode(c.DaemonMode)
	if err != nil {
		return err
	}

	if c.DaemonConfig.FsDriver == FsDriverFscache && daemonMode != DaemonModeShared {
		return errors.New("`fscache` driver only supports `shared` daemon mode")
	}

	if len(c.Root) == 0 {
		return errors.New("empty root directory")
	}

	if c.RemoteConfig.AuthConfig.EnableCRIKeychain && c.RemoteConfig.AuthConfig.EnableKubeconfigKeychain {
		return errors.Wrapf(errdefs.ErrInvalidArgument,
			"\"enable_cri_keychain\" and \"enable_kubeconfig_keychain\" can't be set at the same time")
	}

	if !c.CacheManagerConfig.Disable && c.CacheManagerConfig.CacheDir == "" {
		return errors.Wrapf(errdefs.ErrInvalidArgument, "cache directory is specified to empty")
	}

	return nil
}

// Parse command line arguments and fill the nydus-snapshotter configuration
// Always let options from CLI override those from configuration file.
func ParseParameters(args *command.Args, cfg *SnapshotterConfig) error {
	// --- essential configuration
	cfg.Address = args.Address
	cfg.Root = args.RootDir

	// Give --shared-daemon higher priority
	cfg.DaemonMode = args.DaemonMode

	// --- image processor configuration
	// empty

	// --- daemon configuration
	daemonConfig := &cfg.DaemonConfig
	daemonConfig.NydusdConfigPath = args.NydusdConfigPath
	if args.NydusdPath != "" {
		daemonConfig.NydusdPath = args.NydusdPath
	}
	if args.NydusImagePath != "" {
		daemonConfig.NydusImagePath = args.NydusImagePath
	}
	daemonConfig.FsDriver = args.FsDriver

	// --- cache manager configuration
	// empty

	// --- logging configuration
	logConfig := &cfg.LoggingConfig
	logConfig.LogLevel = args.LogLevel
	logConfig.LogToStdout = args.LogToStdout

	// --- remote storage configuration
	// empty

	// --- snapshot configuration
	// empty

	// --- metrics configuration
	// empty

	return nil
}
