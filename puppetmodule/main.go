package puppetmodule

type DownloadError struct {
	error
	Retryable bool
}

// PuppetModule is implemented by ForgeModule, gitModule, githubTarballModule, ....
type PuppetModule interface {
	Download(to string, cache string) *DownloadError
	InstallPath() string
	IsUpToDate(folder string) bool
	Name() string
}
