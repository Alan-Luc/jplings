package corpus

import (
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Upstream locations and the licences they are used under. JMdict's licence requires
// attribution and a procedure for keeping the data current, so both travel with the
// exported files rather than living only in this source file.
const (
	JMdictURL  = "http://ftp.edrdg.org/pub/Nihongo/JMdict_e.gz"
	JMdictName = "JMdict"
	JMdictLic  = "CC BY-SA 4.0"
	JMdictAttr = "Electronic Dictionary Research and Development Group (EDRDG) — https://www.edrdg.org/wiki/index.php/JMdict-EDICT_Dictionary_Project"

	TatoebaJpn  = "https://downloads.tatoeba.org/exports/per_language/jpn/jpn_sentences.tsv.bz2"
	TatoebaLink = "https://downloads.tatoeba.org/exports/per_language/jpn/jpn-eng_links.tsv.bz2"
	TatoebaEng  = "https://downloads.tatoeba.org/exports/per_language/eng/eng_sentences.tsv.bz2"
	TatoebaName = "Tatoeba"
	TatoebaLic  = "CC BY 2.0 FR"
	TatoebaAttr = "Tatoeba contributors — https://tatoeba.org"
)

// Files are the local paths of the downloaded sources.
type Files struct {
	JMdict   string
	Japanese string
	Links    string
	English  string
}

// Fetch returns the local paths for each source, downloading anything missing into
// cacheDir. Re-running is how the data is kept current, which the JMdict licence
// requires.
func Fetch(cacheDir string) (Files, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return Files{}, err
	}

	files := Files{
		JMdict:   filepath.Join(cacheDir, "JMdict_e.gz"),
		Japanese: filepath.Join(cacheDir, "jpn_sentences.tsv.bz2"),
		Links:    filepath.Join(cacheDir, "jpn-eng_links.tsv.bz2"),
		English:  filepath.Join(cacheDir, "eng_sentences.tsv.bz2"),
	}

	for _, item := range []struct{ path, url string }{
		{files.JMdict, JMdictURL},
		{files.Japanese, TatoebaJpn},
		{files.Links, TatoebaLink},
		{files.English, TatoebaEng},
	} {
		if err := download(item.path, item.url); err != nil {
			return Files{}, err
		}
	}
	return files, nil
}

func download(path, url string) error {
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, response.Status)
	}

	partial := path + ".part"
	file, err := os.Create(partial)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		os.Remove(partial)
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(partial, path)
}

// OpenJMdict decompresses the JMdict dump.
func OpenJMdict(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	return &combinedCloser{Reader: reader, closers: []io.Closer{reader, file}}, nil
}

// OpenBzip2 decompresses a Tatoeba export.
func OpenBzip2(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &combinedCloser{Reader: bzip2.NewReader(file), closers: []io.Closer{file}}, nil
}

type combinedCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *combinedCloser) Close() error {
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// VocabFile is the exported vocabulary reference.
type VocabFile struct {
	Corpus      string  `yaml:"corpus"`
	License     string  `yaml:"license"`
	Attribution string  `yaml:"attribution"`
	Source      string  `yaml:"source_url"`
	Words       []*Word `yaml:"words"`
}

// SentencesFile is the exported sentence reference.
type SentencesFile struct {
	Corpus      string     `yaml:"corpus"`
	License     string     `yaml:"license"`
	Attribution string     `yaml:"attribution"`
	Source      string     `yaml:"source_url"`
	Sentences   []Sentence `yaml:"sentences"`
}

// WriteVocab exports words with their provenance attached.
func WriteVocab(w io.Writer, words []*Word) error {
	return encode(w, VocabFile{
		Corpus: JMdictName, License: JMdictLic, Attribution: JMdictAttr,
		Source: JMdictURL, Words: words,
	})
}

// WriteSentences exports pairs with their provenance attached.
func WriteSentences(w io.Writer, sentences []Sentence) error {
	return encode(w, SentencesFile{
		Corpus: TatoebaName, License: TatoebaLic, Attribution: TatoebaAttr,
		Source: TatoebaJpn, Sentences: sentences,
	})
}

func encode(w io.Writer, value any) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	return encoder.Close()
}
