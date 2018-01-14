package puppetmodule

type DownloadError struct {
	error
	Retryable bool
}

// puppetModule is implemented by ForgeModule, gitModule, githubTarballModule, ....
type PuppetModule interface {
	IsUpToDate(folder string) bool
	GetName() string
	Download(to string, cache string) *DownloadError
	GetInstallPath() string
}
