package policy_test

import (
	"gopkg.in/check.v1"

	"github.com/snapcore/snapd/bootloader"
	"github.com/snapcore/snapd/bootloader/bootloadertest"
	"github.com/snapcore/snapd/dirs"
	"github.com/snapcore/snapd/overlord/snapstate"
	"github.com/snapcore/snapd/overlord/snapstate/policy"
	"github.com/snapcore/snapd/overlord/state"
	"github.com/snapcore/snapd/snap"
	"github.com/snapcore/snapd/snap/snaptest"
	"github.com/snapcore/snapd/testutil"
)

type canRemoveSuite struct {
	testutil.BaseTest
	st *state.State

	bootloader *bootloadertest.MockBootloader
}

var _ = check.Suite(&canRemoveSuite{})

func (s *canRemoveSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	dirs.SetRootDir(c.MkDir())
	s.st = state.New(nil)

	s.bootloader = bootloadertest.Mock("mock", c.MkDir())
	bootloader.Force(s.bootloader)
	s.BaseTest.AddCleanup(snap.MockSanitizePlugsSlots(func(snapInfo *snap.Info) {}))
}

func (s *canRemoveSuite) TearDownTest(c *check.C) {
	dirs.SetRootDir("/")
	bootloader.Force(nil)
	s.BaseTest.TearDownTest(c)
}

func (s *canRemoveSuite) TestAppAreOK(c *check.C) {
	snapst := &snapstate.SnapState{}
	c.Check(policy.NewAppPolicy().CanRemove(s.st, snapst, snap.R(0)), check.IsNil)
	c.Check(policy.NewAppPolicy().CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
}

func (s *canRemoveSuite) TestRequiredAppIsNotOK(c *check.C) {
	snapst := &snapstate.SnapState{Flags: snapstate.Flags{Required: true}}
	c.Check(policy.NewAppPolicy().CanRemove(s.st, snapst, snap.R(0)), check.Equals, policy.ErrRequired)
	c.Check(policy.NewAppPolicy().CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
}

func (s *canRemoveSuite) TestOneGadgetRevisionIsOK(c *check.C) {
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "gadget"}},
	}
	c.Check(policy.NewGadgetPolicy("gadget").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
}

func (s *canRemoveSuite) TestOtherGadgetIsOK(c *check.C) {
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "gadget"}},
	}
	c.Check(policy.NewGadgetPolicy("gadget2").CanRemove(s.st, snapst, snap.R(0)), check.IsNil)
}

func (s *canRemoveSuite) TestLastGadgetsAreNotOK(c *check.C) {
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "gadget"}},
	}
	c.Check(policy.NewGadgetPolicy("gadget").CanRemove(s.st, snapst, snap.R(0)), check.Equals, policy.ErrIsModel)
}

func (s *canRemoveSuite) TestLastOSAndKernelAreNotOK(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "kernel"}},
	}
	// model base is "" -> OS can't be removed
	c.Check(policy.NewOSPolicy("").CanRemove(s.st, snapst, snap.R(0)), check.Equals, policy.ErrIsModel)
	// (well, single revisions are ok)
	c.Check(policy.NewOSPolicy("").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
	// model kernel == snap kernel -> can't be removed
	c.Check(policy.NewKernelPolicy("kernel").CanRemove(s.st, snapst, snap.R(0)), check.Equals, policy.ErrIsModel)
	// (well, single revisions are ok)
	c.Check(policy.NewKernelPolicy("kernel").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
}

func (s *canRemoveSuite) TestOSInUseNotOK(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "core"}},
	}
	// normally this would be fine
	c.Check(policy.NewOSPolicy("").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
	// but not if it's the one we booted
	s.bootloader.SetBootBase("core_1.snap")
	c.Check(policy.NewOSPolicy("").CanRemove(s.st, snapst, snap.R(1)), check.Equals, policy.ErrInUseForBoot)
}

func (s *canRemoveSuite) TestOSRequiredNotOK(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "core"}},
		Flags:    snapstate.Flags{Required: true},
	}
	// can't remove them all if they're required
	c.Check(policy.NewOSPolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.Equals, policy.ErrRequired)
	// but a single rev is ok
	c.Check(policy.NewOSPolicy("core18").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
}

func (s *canRemoveSuite) TestOSUbuntuCoreOK(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "ubuntu-core"}},
	}
	c.Check(policy.NewOSPolicy("").CanRemove(s.st, snapst, snap.R(0)), check.IsNil)
}

func (s *canRemoveSuite) TestKernelBootInUseIsKept(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "kernel"}},
	}

	s.bootloader.SetBootKernel("kernel_1.snap")

	c.Check(policy.NewKernelPolicy("kernel").CanRemove(s.st, snapst, snap.R(1)), check.Equals, policy.ErrInUseForBoot)
}

func (s *canRemoveSuite) TestBaseInUseIsKept(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "core18"}},
	}
	// if not used for boot, removing a single one is ok
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
	// but not all
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.Equals, policy.ErrIsModel)

	// if in use for boot, not even one
	s.bootloader.SetBootBase("core18_1.snap")
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(1)), check.Equals, policy.ErrInUseForBoot)
}

func (s *canRemoveSuite) TestRemoveNonModelKernelIsOk(c *check.C) {
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "other-non-model-kernel"}},
	}

	c.Check(policy.NewKernelPolicy("kernel").CanRemove(s.st, snapst, snap.R(0)), check.IsNil)
}

func (s *canRemoveSuite) TestLastOSWithModelBaseIsOk(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "core"}},
	}

	c.Check(policy.NewOSPolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.IsNil)
}

func (s *canRemoveSuite) TestLastOSWithModelBaseButOsInUse(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	// pretend we have a snap installed that has no base (which means
	// it needs core)
	si := &snap.SideInfo{RealName: "some-snap", SnapID: "some-snap-id", Revision: snap.R(1)}
	snaptest.MockSnap(c, "name: some-snap\nversion: 1.0", si)
	snapstate.Set(s.st, "some-snap", &snapstate.SnapState{
		Sequence: []*snap.SideInfo{si},
		Current:  snap.R(1),
	})

	// now pretend we want to remove the core snap
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "core"}},
	}
	c.Check(policy.NewOSPolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.DeepEquals, policy.InUseByErr("some-snap"))
}

func (s *canRemoveSuite) TestLastOSWithModelBaseButOsInUseByGadget(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	// pretend we have a gadget snap installed that has no base (which means
	// it needs core)
	si := &snap.SideInfo{RealName: "some-gadget", SnapID: "some-gadget-id", Revision: snap.R(1)}
	snaptest.MockSnap(c, "name: some-gadget\ntype: gadget\nversion: 1.0", si)
	snapstate.Set(s.st, "some-gadget", &snapstate.SnapState{
		Sequence: []*snap.SideInfo{si},
		Current:  snap.R(1),
		SnapType: "gadget",
	})

	// now pretend we want to remove the core snap
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "core"}},
	}
	c.Check(policy.NewOSPolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.DeepEquals, policy.InUseByErr("some-gadget"))
}

func (s *canRemoveSuite) TestBaseUnused(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "foo"}},
	}

	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.IsNil)
}

func (s *canRemoveSuite) TestBaseUnusedButRequired(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "foo"}},
		Flags:    snapstate.Flags{Required: true},
	}

	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(1)), check.IsNil)
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.Equals, policy.ErrRequired)
}

func (s *canRemoveSuite) TestBaseInUse(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	// pretend we have a snap installed that uses "some-base"
	si := &snap.SideInfo{RealName: "some-snap", SnapID: "some-snap-id", Revision: snap.R(1)}
	snaptest.MockSnap(c, "name: some-snap\nversion: 1.0\nbase: some-base", si)
	snapstate.Set(s.st, "some-snap", &snapstate.SnapState{
		Sequence: []*snap.SideInfo{si},
		Current:  snap.R(1),
	})

	// pretend now we want to remove "some-base"
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "some-base"}},
	}
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.DeepEquals, policy.InUseByErr("some-snap"))
}

func (s *canRemoveSuite) TestBaseInUseBrokenApp(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	// pretend we have a snap installed that uses "some-base"
	si := &snap.SideInfo{RealName: "some-snap", SnapID: "some-snap-id", Revision: snap.R(1)}
	si2 := &snap.SideInfo{RealName: "some-snap", SnapID: "some-snap-id", Revision: snap.R(2)}
	snaptest.MockSnap(c, "name: some-snap\nversion: 1.0\nbase: some-base", si)
	// NOTE no snaptest.MockSnap for si2 -> snap is actually broken
	snapstate.Set(s.st, "some-snap", &snapstate.SnapState{
		Sequence: []*snap.SideInfo{si2, si},
		Current:  snap.R(2),
	})

	// pretend now we want to remove "some-base"
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "some-base"}},
	}
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.DeepEquals, policy.InUseByErr("some-snap"))
}

func (s *canRemoveSuite) TestBaseInUseOtherRevision(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	// pretend we have a snap installed that uses "some-base"
	si := &snap.SideInfo{RealName: "some-snap", SnapID: "some-snap-id", Revision: snap.R(1)}
	si2 := &snap.SideInfo{RealName: "some-snap", SnapID: "some-snap-id", Revision: snap.R(2)}
	// older revision uses base
	snaptest.MockSnap(c, "name: some-snap\nversion: 1.0\nbase: some-base", si)
	// new one does not
	snaptest.MockSnap(c, "name: some-snap\nversion: 1.0\n", si2)
	snapstate.Set(s.st, "some-snap", &snapstate.SnapState{
		Active:   true,
		Sequence: []*snap.SideInfo{si, si2},
		Current:  snap.R(2),
	})

	// pretend now we want to remove "some-base"
	snapst := &snapstate.SnapState{
		Current:  snap.R(1),
		Sequence: []*snap.SideInfo{{Revision: snap.R(1), RealName: "some-base"}},
	}
	// revision 1 requires some-base
	c.Check(policy.NewBasePolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.DeepEquals, policy.InUseByErr("some-snap"))

	// now pretend we want to remove the core snap
	snapst.Sequence[0].RealName = "core"
	// but revision 2 requires core
	c.Check(policy.NewOSPolicy("core18").CanRemove(s.st, snapst, snap.R(0)), check.DeepEquals, policy.InUseByErr("some-snap"))
}
