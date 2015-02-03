package zfs_test

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-check"
	"github.com/flynn/flynn/host/volume"
	"github.com/flynn/flynn/host/volume/manager"
	"github.com/flynn/flynn/host/volume/zfs"
	"github.com/flynn/flynn/pkg/random"
	"github.com/flynn/flynn/pkg/testutils"
)

// note: many of these tests are not zfs specific; refactoring this when we have more concrete backends will be wise.

type PersistenceTests struct{}

var _ = Suite(&PersistenceTests{})

func (PersistenceTests) SetUpSuite(c *C) {
	// Skip all tests in this suite if not running as root.
	// Many zfs operations require root priviledges.
	testutils.SkipIfNotRoot(c)
}

func (s *PersistenceTests) TestApplePie(c *C) {
	stringOfDreams := random.String(12)
	vmanDBfilePath := fmt.Sprintf("/tmp/flynn-volumes-%s.bold", stringOfDreams)
	zfsDatasetName := fmt.Sprintf("flynn-test-dataset-%s", stringOfDreams)

	// new volume manager with a new backing zfs vdev file and a new boltdb
	volProv, err := zfs.NewProvider(&zfs.ProviderConfig{
		DatasetName: zfsDatasetName,
		Make: &zfs.MakeDev{
			BackingFilename: fmt.Sprintf("/tmp/flynn-test-zpool-%s.vdev", stringOfDreams),
			Size:            int64(math.Pow(2, float64(30))),
		},
	})
	c.Assert(err, IsNil)

	// new volume manager with that shiney new backing zfs vdev file and a new boltdb
	vman, err := volumemanager.New(
		vmanDBfilePath,
		func() (volume.Provider, error) { return volProv, nil },
	)
	c.Assert(err, IsNil)

	// make a volume
	vol1, err := vman.NewVolume()
	c.Assert(err, IsNil)

	// make a named volume
	vol2, err := vman.CreateOrGetNamedVolume("aname", "")
	c.Assert(err, IsNil)

	// assert existence of filesystems; emplace some data
	vol1path, err := vol1.Mount("mock", "irrelevant")
	c.Assert(err, IsNil)
	f, err := os.Create(filepath.Join(vol1path, "alpha"))
	c.Assert(err, IsNil)
	f.Close()
	vol2path, err := vol2.Mount("mock", "irrelevant")
	c.Assert(err, IsNil)
	f, err = os.Create(filepath.Join(vol2path, "beta"))
	c.Assert(err, IsNil)
	f.Close()

	// close persistence
	c.Assert(vman.PersistenceDBClose(), IsNil)

	// hack zfs export/umounting to emulate host shutdown
	err = exec.Command("zpool", "export", "-f", zfsDatasetName).Run()
	c.Assert(err, IsNil)
	// TODO: this should *ALSO* work when all the mounts are still there.
	// or some.
	// or if there are extras.
	// so, there should be tables driving here, basically.

	// restore
	vman, err = volumemanager.New(
		vmanDBfilePath,
		func() (volume.Provider, error) {
			c.Fatal("default provider setup should not be called if the previous provider was restored")
			return nil, nil
		},
	)
	c.Assert(err, IsNil)

	// assert volumes
	restoredVolumes := vman.Volumes()
	c.Assert(restoredVolumes, HasLen, 2)
	c.Assert(restoredVolumes[vol1.Info().ID], NotNil)
	c.Assert(restoredVolumes[vol2.Info().ID], NotNil)

	// assert named volumes

	// assert existences of filesystems and previous data
	vol1path, err = vol1.Mount("mock", "irrelevant")
	c.Assert(err, IsNil)
	c.Assert(vol1path, testutils.DirContains, []string{"alpha"})
	vol2path, err = vol2.Mount("mock", "irrelevant")
	c.Assert(err, IsNil)
	c.Assert(vol2path, testutils.DirContains, []string{"beta"})

}
