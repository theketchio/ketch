package archive

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

const archiveFileName = "archive.tar.gz"

func TestCreate(t *testing.T) {
	root := tree{
		dir: "dir",
		files: []file{
			{"file1.txt", "file1-data"},
		},
		subs: []tree{
			{
				dir: "dir2",
				files: []file{
					{"file2.txt", "file2-data"},
					{"file3.txt", "file3-data"},
				},
			},
		},
	}
	testPath := t.TempDir()
	err := setupDirectoryStructure(testPath, root)
	require.Nil(t, err)
	archiveFile := path.Join(testPath, archiveFileName)
	err = Create(archiveFileName, IncludeDirs("dir"), WithWorkingDirectory(testPath))
	require.Nil(t, err)
	_, err = os.Stat(archiveFile)
	require.Nil(t, err)
	err = os.Chdir(testPath)
	require.Nil(t, err)
	// remove the directories that were tarred up, and unpack the tarfile.
	err = os.RemoveAll(root.dir)
	require.Nil(t, err)
	cmd := exec.Command("tar", "-xzf", archiveFileName)
	err = cmd.Run()
	require.Nil(t, err)
	// the same directories and files should be re-created
	err = evaluateResults(testPath, root)
	require.Nil(t, err, "unexpected error %v", err)
}

func TestCreateWithIgnoredFiles(t *testing.T) {
	root := tree{
		files: []file{
			{ignoreFile, "*.foo"},
		},
		subs: []tree{
			{
				dir: "dir",
				files: []file{
					{"file.foo", "file-foo"},
					{"file.txt", "file-txt"},
				},
			},
		},
	}
	expected := tree{
		files: []file{
			{ignoreFile, "*.foo"},
		},
		subs: []tree{
			{
				dir: "dir",
				files: []file{
					{"file.txt", "file-txt"},
				},
			},
		},
	}
	workingDir := t.TempDir()
	require.Nil(t, setupDirectoryStructure(workingDir, root))
	require.Nil(t, setHomeToWd())
	err := Create(archiveFileName, WithWorkingDirectory(workingDir))
	require.Nil(t, err)
	require.Nil(t, os.Chdir(workingDir))
	require.Nil(t, os.RemoveAll(ignoreFile))
	require.Nil(t, os.RemoveAll("dir"))
	require.Nil(t, exec.Command("tar", "-xzf", archiveFileName).Run())
	require.Nil(t, evaluateResults(workingDir, expected))
}

// this is a dirty hack to set the working directory to a directory that we know exists.
// otherwise os.Getwd fails unpredictably on some tests
func setHomeToWd() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	os.Chdir(u.HomeDir)
	return nil
}

func TestCreateWithIncludedFiles(t *testing.T) {
	root := tree{
		files: []file{
			{ignoreFile, "*.foo"},
		},
		subs: []tree{
			{
				dir: "dir0",
				files: []file{
					{"file.txt", "some data"},
					{"file.foo", "foo data"},
				},
				subs: []tree{
					{
						dir: "dir1",
						files: []file{
							{"file1.txt", "file1 data"},
						},
					},
				},
			},
		},
	}
	expected := tree{
		subs: []tree{
			{
				dir: "dir0",
				files: []file{
					{"file.foo", "foo data"},
				},
				subs: []tree{
					{
						dir: "dir1",
						files: []file{
							{"file1.txt", "file1 data"},
						},
					},
				},
			},
		},
	}
	workingDir := t.TempDir()
	require.Nil(t, setupDirectoryStructure(workingDir, root))
	require.Nil(t, setHomeToWd())
	require.Nil(t, Create(
		archiveFileName,
		WithWorkingDirectory(workingDir),
		IncludeFiles("dir0/file.foo", "dir0/dir1/file1.txt")),
	)
	require.Nil(t, os.Chdir(workingDir))
	require.Nil(t, os.RemoveAll(ignoreFile))
	require.Nil(t, os.RemoveAll("dir0"))
	require.Nil(t, exec.Command("tar", "-xzf", archiveFileName).Run())
	require.Nil(t, evaluateResults(workingDir, expected))
}

type file struct {
	name string
	data string
}

type tree struct {
	dir   string
	subs  []tree
	files []file
}

func evaluateResults(parentDir string, root tree) error {
	if root.dir != "" {
		parentDir = path.Join(parentDir, root.dir)
	}
	for _, file := range root.files {
		filePath := path.Join(parentDir, file.name)
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("file %s not opened %w", filePath, err)
		}
		buff, err := ioutil.ReadAll(f)
		if err != nil {
			return fmt.Errorf("could not read %s error %w", filePath, err)
		}
		f.Close()
		if string(buff) != file.data {
			return fmt.Errorf("data mismatch for %q want %q got %q", filePath, file.data, string(buff))
		}
	}

	for _, subTree := range root.subs {
		if err := evaluateResults(parentDir, subTree); err != nil {
			return err
		}
	}
	return nil
}

func setupDirectoryStructure(parentDir string, root tree) error {
	if root.dir != "" {
		parentDir = path.Join(parentDir, root.dir)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return err
		}
	}
	for _, file := range root.files {
		filePath := path.Join(parentDir, file.name)
		f, err := os.Create(filePath)
		if err != nil {
			return err
		}
		if _, err := f.WriteString(file.data); err != nil {
			return err
		}
		f.Close()
	}

	for _, subTree := range root.subs {
		if err := setupDirectoryStructure(parentDir, subTree); err != nil {
			return err
		}
	}
	return nil
}
