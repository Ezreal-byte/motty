package remotetools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

type shellRunner struct{ commands []string }

func (runner *shellRunner) RunCommand(ctx context.Context, command string, limit int) ([]byte, error) {
	runner.commands = append(runner.commands, command)
	output, err := exec.CommandContext(ctx, "/bin/sh", "-c", command).Output()
	return output, err
}

type fixedRunner struct {
	output string
	err    error
	asked  string
}

func (runner *fixedRunner) RunCommand(_ context.Context, command string, _ int) ([]byte, error) {
	runner.asked = command
	return []byte(runner.output), runner.err
}

func TestParsingTheProcessListSortsAndSkipsItsOwnPipeline(t *testing.T) {
	output := "" +
		"    1 root      0.1  0.2    10-02:03:04 /sbin/init splash\n" +
		" 2211 www-data  42.5 1.3       01:02:03 nginx: worker process   (with   spaces)\n" +
		" 3000 alice      7.0 30.9          05:01 /usr/bin/python3 /srv/app.py --flag\n" +
		" 9001 alice      0.0  0.0          00:00 sh -c LC_ALL=C ps -eo pid=,user=,pcpu=,pmem=,etime=,args= 2>&1 | sort\n" +
		" 9003 alice      0.0  0.0          00:00 sort -k3,3nr\n" +
		" 9004 alice      0.0  0.0          00:00 head -n 68\n"
	byCPU, err := ParseProcesses([]byte(output), SortCPU)
	if err != nil || len(byCPU) != 3 {
		t.Fatalf("%d processes, %v", len(byCPU), err)
	}
	if byCPU[0].PID != 2211 || byCPU[0].CPU != 42.5 || byCPU[0].User != "www-data" || byCPU[0].Elapsed != "01:02:03" ||
		byCPU[0].Command != "nginx: worker process   (with   spaces)" {
		t.Fatalf("first by CPU: %+v", byCPU[0])
	}
	byMemory, _ := ParseProcesses([]byte(output), SortMemory)
	if byMemory[0].PID != 3000 {
		t.Fatalf("first by memory: %+v", byMemory[0])
	}
}

func TestAListingThatIsAnErrorMessageIsReportedAsUnsupported(t *testing.T) {
	_, err := ParseProcesses([]byte("ps: invalid option -- 'e'\nBusyBox v1.35\n"), SortCPU)
	var failure *Failure
	if !errors.Is(err, ErrUnsupported) || !errors.As(err, &failure) || !strings.Contains(failure.Message, "invalid option") {
		t.Fatalf("got %v", err)
	}
	if _, err := ParseProcesses(nil, SortCPU); !errors.Is(err, ErrMalformed) {
		t.Fatalf("empty output: %v", err)
	}
}

func TestTheProcessListRunsOnThisComputer(t *testing.T) {
	if _, err := exec.LookPath("ps"); err != nil {
		t.Skip("no ps")
	}
	runner := &shellRunner{}
	list, err := Processes(context.Background(), runner, SortMemory)
	if err != nil || len(list) == 0 {
		t.Fatalf("%d processes, %v", len(list), err)
	}
	if len(list) > maxProcesses {
		t.Fatalf("%d processes returned", len(list))
	}
	for _, process := range list {
		if ownPipeline(process.Command) {
			t.Fatalf("the listing's own process is in it: %+v", process)
		}
	}
}

func TestSignalsRefuseWhatCouldReachMoreThanOneProcess(t *testing.T) {
	runner := &fixedRunner{}
	for _, pid := range []int{-1, 0, 1} {
		if err := SendSignal(context.Background(), runner, pid, Kill); err == nil {
			t.Fatalf("pid %d was accepted", pid)
		}
	}
	if runner.asked != "" {
		t.Fatalf("a command was sent: %q", runner.asked)
	}
	if err := SendSignal(context.Background(), runner, 4242, Terminate); err != nil || runner.asked != "kill -TERM 4242 2>&1" {
		t.Fatalf("%q, %v", runner.asked, err)
	}
	runner.err, runner.output = errors.New("exit status 1"), "sh: line 1: kill: (4242) - Operation not permitted\nmore\n"
	err := SendSignal(context.Background(), runner, 4242, Kill)
	var failure *Failure
	if !errors.As(err, &failure) || !strings.Contains(failure.Message, "Operation not permitted") || strings.Contains(failure.Message, "more") {
		t.Fatalf("got %v", err)
	}
}

func TestParsingTheContainerListPutsRunningOnesFirst(t *testing.T) {
	output := "" +
		"a1b2c3d4e5f6|old-job|alpine:3.19|exited|Exited (0) 3 days ago\n" +
		"0f1e2d3c4b5a|web|nginx:latest|running|Up 2 hours (healthy)\n"
	list, err := ParseContainers([]byte(output))
	if err != nil || len(list) != 2 || list[0].Name != "web" || list[0].Status != "Up 2 hours (healthy)" || !list[0].Running() || list[1].Running() {
		t.Fatalf("%+v, %v", list, err)
	}
	if _, err := ParseContainers([]byte("Cannot connect to the Docker daemon\n")); !errors.Is(err, ErrMalformed) {
		t.Fatalf("a message was read as a container: %v", err)
	}
}

func TestContainersExplainAMissingOrRefusedDocker(t *testing.T) {
	missing := &fixedRunner{output: "sh: 1: docker: not found\n", err: errors.New("exit status 127")}
	if _, err := Containers(context.Background(), missing); !errors.Is(err, ErrNoDocker) {
		t.Fatalf("missing docker: %v", err)
	}
	refused := &fixedRunner{output: "permission denied while trying to connect to the Docker daemon socket\nsecond line\n", err: errors.New("exit status 1")}
	_, err := Containers(context.Background(), refused)
	var failure *Failure
	if !errors.As(err, &failure) || !strings.HasPrefix(failure.Message, "permission denied") || strings.Contains(failure.Message, "second") {
		t.Fatalf("refused: %v", err)
	}
}

func TestContainerActionsOnlyTakeValidNames(t *testing.T) {
	runner := &fixedRunner{}
	for _, id := range []string{"", "web; rm -rf /", "$(reboot)", "a b", "-rf", strings.Repeat("a", 200), "web\nx"} {
		if err := Do(context.Background(), runner, Stop, id); err == nil {
			t.Fatalf("%q was accepted", id)
		}
		if _, ok := LogsCommand(id); ok {
			t.Fatalf("logs for %q", id)
		}
	}
	if runner.asked != "" {
		t.Fatalf("a command was sent: %q", runner.asked)
	}
	for action, want := range map[Action]string{Start: "docker start web-1 2>&1", Stop: "docker stop web-1 2>&1", Restart: "docker restart web-1 2>&1", Remove: "docker rm web-1 2>&1"} {
		if err := Do(context.Background(), runner, action, "web-1"); err != nil || runner.asked != want {
			t.Fatalf("%q, %v", runner.asked, err)
		}
	}
	if command, ok := LogsCommand("web-1"); !ok || command != "docker logs -f --tail 200 web-1" {
		t.Fatalf("logs: %q", command)
	}
}

func TestDockerObjectListingsBecomeRowsKeyedByWhatRemovesThem(t *testing.T) {
	images, err := ParseResources(Images, []byte("d4c45f66efc5|debian:bookworm-slim|74.9MB|2 weeks ago\ncf001c2f8af7|docker.m.example.com/library/python:<none>|1GB|3 months ago\n"), 4)
	if err != nil || len(images) != 2 {
		t.Fatalf("%d images, %v", len(images), err)
	}
	if images[0].Key != "debian:bookworm-slim" || images[0].Cells[0] != "debian:bookworm-slim" || images[0].Cells[1] != "d4c45f66efc5" {
		t.Errorf("a tagged image: %+v", images[0])
	}
	if images[1].Key != "cf001c2f8af7" {
		t.Errorf("an untagged image must be removed by its ID, got %q", images[1].Key)
	}
	volumes, err := ParseResources(Volumes, []byte("pgdata|local|local\n"), 3)
	if err != nil || volumes[0].Key != "pgdata" {
		t.Errorf("volume: %+v, %v", volumes, err)
	}
	networks, err := ParseResources(Networks, []byte("4cb549b7e5e0|app_default|bridge|local\n"), 4)
	if err != nil || networks[0].Key != "4cb549b7e5e0" || networks[0].Cells[0] != "app_default" {
		t.Errorf("network: %+v, %v", networks, err)
	}
	if _, err := ParseResources(Volumes, []byte("only|two\n"), 3); !errors.Is(err, ErrMalformed) {
		t.Errorf("a short line: %v", err)
	}
	if _, err := ParseResources(Volumes, []byte("a b; rm -rf /|local|local\n"), 3); !errors.Is(err, ErrMalformed) {
		t.Errorf("a name that could become shell syntax was accepted: %v", err)
	}
}

func TestRemovingAnObjectRunsOneFixedCommandAndRefusesUnsafeNames(t *testing.T) {
	runner := &fixedRunner{}
	for kind, want := range map[Kind]string{Images: "docker rmi web:1 2>&1", Volumes: "docker volume rm web:1 2>&1", Networks: "docker network rm web:1 2>&1"} {
		if err := RemoveResource(context.Background(), runner, kind, "web:1"); err != nil || runner.asked != want {
			t.Errorf("kind %d ran %q (%v), want %q", kind, runner.asked, err, want)
		}
	}
	runner.asked = ""
	if err := RemoveResource(context.Background(), runner, Images, "x; reboot"); err == nil || runner.asked != "" {
		t.Errorf("an unsafe name reached the server: %q, %v", runner.asked, err)
	}
}

func TestNamedVolumesComeBeforeTheAnonymousOnesAndSurviveTheLimit(t *testing.T) {
	var listing strings.Builder
	for index := 0; index < maxItems+50; index++ {
		listing.WriteString(strings.Repeat("a", 60) + fmt.Sprintf("%04d", index) + "|local|local\n")
	}
	listing.WriteString("pgdata|local|local\n")
	volumes, err := ParseResources(Volumes, []byte(listing.String()), 3)
	if err != nil || len(volumes) != maxItems {
		t.Fatalf("%d volumes, %v", len(volumes), err)
	}
	if volumes[0].Key != "pgdata" {
		t.Errorf("the named volume is at the end, past the limit: first is %q", volumes[0].Key)
	}
}

func TestParseComposeReadsTheJSONListing(t *testing.T) {
	projects, err := ParseCompose([]byte(`[{"Name":"web","Status":"running(2)","ConfigFiles":"/a/compose.yml,/a/o.yml"},{"Name":"api","Status":"exited(1)","ConfigFiles":"/b/compose.yml"}]`))
	if err != nil || len(projects) != 2 || projects[0].Name != "api" || projects[1].Status != "running(2)" {
		t.Fatalf("%+v, %v", projects, err)
	}
	if projects, err := ParseCompose([]byte("[]\n")); err != nil || len(projects) != 0 {
		t.Errorf("empty: %+v, %v", projects, err)
	}
	if _, err := ParseCompose([]byte("not json")); !errors.Is(err, ErrMalformed) {
		t.Errorf("garbage: %v", err)
	}
}

func TestComposeNeedsThePluginAndActionsRefuseUnsafeNames(t *testing.T) {
	runner := &fixedRunner{output: "docker: 'compose' is not a docker command.\nSee 'docker --help'", err: errors.New("exit status 1")}
	if _, err := Compose(context.Background(), runner); !errors.Is(err, ErrNoCompose) {
		t.Errorf("no plugin: %v", err)
	}
	runner = &fixedRunner{}
	if err := DoCompose(context.Background(), runner, ComposeDown, "my-app_1"); err != nil || runner.asked != "docker compose -p my-app_1 down 2>&1" {
		t.Errorf("ran %q, %v", runner.asked, err)
	}
	runner.asked = ""
	for _, name := range []string{"", "Web", "a b", "x;reboot", "-f", "$(id)"} {
		if err := DoCompose(context.Background(), runner, ComposeStart, name); err == nil || runner.asked != "" {
			t.Errorf("%q reached the server: %q", name, runner.asked)
		}
	}
}

func TestReniceIsOneFixedCommandWithBoundedNumbers(t *testing.T) {
	runner := &fixedRunner{}
	if err := Renice(context.Background(), runner, 4321, -5); err != nil || runner.asked != "renice -5 -p 4321 2>&1" {
		t.Errorf("ran %q, %v", runner.asked, err)
	}
	runner.asked = ""
	for _, bad := range [][2]int{{1, 0}, {0, 0}, {-3, 0}, {10, 20}, {10, -21}} {
		if err := Renice(context.Background(), runner, bad[0], bad[1]); err == nil || runner.asked != "" {
			t.Errorf("%v reached the server: %q", bad, runner.asked)
		}
	}
}

func TestCreatingAVolumeOrNetworkRunsOneFixedCommandAndRefusesUnsafeNames(t *testing.T) {
	runner := &fixedRunner{}
	for kind, want := range map[Kind]string{Volumes: "docker volume create data_1 2>&1", Networks: "docker network create data_1 2>&1"} {
		if err := CreateResource(context.Background(), runner, kind, "data_1"); err != nil || runner.asked != want {
			t.Errorf("kind %d ran %q (%v), want %q", kind, runner.asked, err, want)
		}
	}
	runner.asked = ""
	for _, name := range []string{"x; reboot", "a b", "-v", "web:1", ""} {
		if err := CreateResource(context.Background(), runner, Volumes, name); err == nil || runner.asked != "" {
			t.Errorf("%q reached the server: %q, %v", name, runner.asked, err)
		}
	}
	if err := CreateResource(context.Background(), runner, Images, "nginx"); err == nil || runner.asked != "" {
		t.Errorf("an image was created: %q, %v", runner.asked, err)
	}
}

func TestPullCommandIsOnlyBuiltForAnImageReference(t *testing.T) {
	if command, ok := PullCommand("ghcr.io/org/app:1.2@sha256:abc"); !ok || command != "docker pull ghcr.io/org/app:1.2@sha256:abc" {
		t.Errorf("got %q, %v", command, ok)
	}
	for _, reference := range []string{"nginx; reboot", "a b", "--all-tags", "", "$(id)"} {
		if command, ok := PullCommand(reference); ok {
			t.Errorf("%q became %q", reference, command)
		}
	}
}
