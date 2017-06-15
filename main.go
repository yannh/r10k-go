package main

import "sync"
import "os"
import "log"
import "io"
import "github.com/docopt/docopt-go"

type PuppetModule interface {
	Name() string
  Download() string
  SetTargetFolder(string)
  TargetFolder() string
}

type Parser interface {
  parse(puppetFile io.Reader, modulesChan chan PuppetModule, wg *sync.WaitGroup) error
}

func cloneWorker(c chan PuppetModule, wg *sync.WaitGroup) {
  parser := MetadataParser{}

  for m := range c{
    target :=m.Download()

    if file, err := os.Open(target+"/metadata.json"); err == nil {
      defer file.Close()
     	parser.parse(file, c, wg)
    }

    wg.Done()
  }
}

func main() {
  usage := `r10k-go.

Usage:
  r10k-go deploy environment <env_name>

Options:
  -h --help     Show this screen.`
  numWorkers := 2

  docopt.Parse(usage, nil, true, "r10k-go", false)

  file, err := os.Open("Puppetfile")
  if err != nil {
      log.Fatal(err)
  }
  defer file.Close()

  modulesChan := make(chan PuppetModule)
  var wg sync.WaitGroup

  for w := 1; w <= numWorkers; w++ {
    go cloneWorker(modulesChan, &wg)
  }

  parser := PuppetFileParser{}
	parser.parse(file, modulesChan, &wg)

  wg.Wait()
  close(modulesChan)
}
