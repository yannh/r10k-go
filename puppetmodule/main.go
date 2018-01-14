package puppetmodule

type DownloadError struct {
	error
	Retryable bool
}

// PuppetModule is implemented by ForgeModule, gitModule, githubTarballModule, ....
type PuppetModule interface {
	IsUpToDate(folder string) bool
	Name() string
	Download(to string, cache string) *DownloadError
	GetInstallPath() string
}
