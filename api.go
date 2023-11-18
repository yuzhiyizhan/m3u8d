package m3u8d

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var gMergeStatus SpeedStatus
var gMergeCancel context.CancelFunc
var gMergeCancelLocker sync.Mutex

type MergeTsDir_Resp struct {
	ErrMsg   string
	IsCancel bool
}

var gMergeIsRunning atomic.Bool

func MergeTsDir(InputTsDir string, OutputMp4Name string) (resp MergeTsDir_Resp) {
	if !gMergeIsRunning.CompareAndSwap(false, true) {
		return resp
	}
	defer gMergeIsRunning.Store(false)

	fList, err := ioutil.ReadDir(InputTsDir)
	if err != nil {
		resp.ErrMsg = "读取目录失败 " + err.Error()
		return
	}
	var tsFileList []string
	for _, f := range fList {
		if f.Mode().IsRegular() && strings.HasSuffix(strings.ToLower(f.Name()), ".ts") {
			tsFileList = append(tsFileList, filepath.Join(InputTsDir, f.Name()))
		}
	}
	if len(tsFileList) == 0 {
		resp.ErrMsg = "目录下不存在ts文件: " + InputTsDir
		return
	}
	sort.Strings(tsFileList) // 按照字典顺序排序
	if OutputMp4Name == "" {
		OutputMp4Name = filepath.Join(InputTsDir, "all.mp4")
	}
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	gMergeCancelLocker.Lock()
	if gMergeCancel != nil {
		gMergeCancel()
	}
	gMergeCancel = cancelFn
	gMergeCancelLocker.Unlock()

	err = MergeTsFileListToSingleMp4(MergeTsFileListToSingleMp4_Req{
		TsFileList: tsFileList,
		OutputMp4:  OutputMp4Name,
		Ctx:        ctx,
		Status:     &gMergeStatus,
	})
	if err != nil {
		resp.ErrMsg = "合并错误: " + err.Error()
		resp.IsCancel = isContextCancel(ctx)
		return resp
	}
	return resp
}

type MergeGetProgressPercent_Resp struct {
	Percent   int
	SpeedText string
	IsRunning bool
}

func MergeGetProgressPercent() (resp MergeGetProgressPercent_Resp) {
	resp.IsRunning = gMergeIsRunning.Load()
	if resp.IsRunning {
		resp.Percent = gMergeStatus.GetPercent()
		resp.SpeedText = gMergeStatus.SpeedRecent5sGetAndUpdate()
	}
	return resp
}

func MergeStop() {
	gMergeCancelLocker.Lock()
	if gMergeCancel != nil {
		gMergeCancel()
	}
	gMergeCancelLocker.Unlock()
}