//go:build darwin || linux

package localbudget

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

type inode struct{ device, number uint64 }

type storageWalk struct {
	ctx context.Context
	seen map[inode]bool
	result Storage
}

func collectStorage(ctx context.Context, roots []string) (Storage, error) {
	if err := ctx.Err(); err != nil { return Storage{}, err }
	if len(roots)==0 || len(roots)>16 { return Storage{}, errors.New("explicit bounded roots required") }
	paths := append([]string(nil), roots...)
	for i, path := range paths {
		if !utf8.ValidString(path) || !filepath.IsAbs(path) || filepath.Clean(path)!=path || path=="/" || len(path)>1024 || strings.IndexFunc(path,func(r rune)bool{return r<32||r==127})>=0 {
			return Storage{}, errors.New("invalid storage root")
		}
		for _, prior := range paths[:i] {
			if path==prior || strings.HasPrefix(path,prior+string(filepath.Separator)) || strings.HasPrefix(prior,path+string(filepath.Separator)) {
				return Storage{}, errors.New("overlapping storage roots")
			}
		}
	}
	walk := storageWalk{ctx:ctx,seen:make(map[inode]bool),result:Storage{StartedAt:time.Now().UTC(),Roots:uint64(len(paths))}}
	for _, path := range paths {
		if err:=walk.root(path);err!=nil {return Storage{},err}
	}
	if err:=ctx.Err();err!=nil{return Storage{},err}
	walk.result.FinishedAt=time.Now().UTC()
	return walk.result,nil
}

// Anchor every root component without following links, then verify os.Root
// opened that same directory before using its traversal-resistant operations.
func storageAnchor(path string) (result *os.File, resultErr error) {
	fd,err:=unix.Open("/",unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC,0)
	if err!=nil{return nil,err}
	for _,part:=range strings.Split(strings.TrimPrefix(path,"/"),"/") {
		next,openErr:=unix.Openat(fd,part,unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,0)
		closeErr:=unix.Close(fd)
		if openErr!=nil{return nil,errors.Join(openErr,closeErr)}
		if closeErr!=nil{return nil,errors.Join(closeErr,unix.Close(next))}
		fd=next
	}
	return os.NewFile(uintptr(fd),"storage-anchor"),nil
}

func (w *storageWalk) root(path string) (err error) {
	if err=w.ctx.Err();err!=nil{return err}
	anchor,err:=storageAnchor(path)
	if err!=nil{return err}
	defer func(){err=errors.Join(err,anchor.Close())}()
	before,err:=anchor.Stat()
	if err!=nil{return err}
	stat,ok:=before.Sys().(*syscall.Stat_t)
	if !ok||stat.Uid!=uint32(os.Geteuid()){return errors.New("storage root must belong to operator")}
	root,err:=os.OpenRoot(path)
	if err!=nil{return err}
	defer func(){err=errors.Join(err,root.Close())}()
	opened,err:=root.Stat(".")
	if err!=nil{return err}
	if !os.SameFile(before,opened){return errors.New("storage root changed")}
	if err:=w.visit(root,".",0);err!=nil{return err}
	current,err:=storageAnchor(path)
	if err!=nil{return err}
	defer func(){err=errors.Join(err,current.Close())}()
	after,err:=current.Stat()
	if err!=nil{return err}
	currentStat,ok:=after.Sys().(*syscall.Stat_t)
	if !ok||currentStat.Uid!=uint32(os.Geteuid())||!os.SameFile(before,after){return errors.New("storage root changed during collection")}
	return nil
}

func (w *storageWalk) visit(root *os.Root, path string, depth int) error {
	if err:=w.ctx.Err();err!=nil{return err}
	if depth>64 || w.result.Entries>=50000{return errors.New("storage traversal limit reached")}
	info,err:=root.Lstat(path)
	if err!=nil{return err}
	stat,ok:=info.Sys().(*syscall.Stat_t)
	if !ok||stat.Blocks<0||info.Size()<0{return errors.New("invalid storage metadata")}
	w.result.Entries++ // Bound visited names even when inodes are hard-linked.
	// Validate every link in its own root, even if its inode was counted in
	// another root: hard-linked symlinks can resolve differently by location.
	if info.Mode()&os.ModeSymlink!=0 {
		if _,err:=root.Stat(path);err!=nil{return err}
	}
	key:=inode{uint64(stat.Dev),uint64(stat.Ino)}
	if w.seen[key]{return nil}
	w.seen[key]=true
	blocks:=uint64(stat.Blocks)
	if blocks>math.MaxUint64/512 || blocks*512>math.MaxUint64-w.result.AllocatedBytes{return errors.New("storage total overflow")}
	w.result.AllocatedBytes+=blocks*512
	switch {
	case info.Mode().IsRegular():
		size:=uint64(info.Size())
		if size>math.MaxUint64-w.result.LogicalBytes{return errors.New("storage length overflow")}
		w.result.LogicalBytes+=size
		return nil
	case info.Mode()&os.ModeSymlink!=0:
		// Count link metadata only. Root.Stat verifies that resolving the link
		// stays within this root; the target is counted at its own tree entry.
		return nil
	case info.IsDir():
		return w.directory(root,path,depth,info)
	case info.Mode()&os.ModeSocket!=0:
		return nil // Local socket metadata only; never connect/open the socket.
	default:
		return errors.New("unsupported special file in storage root")
	}
}

func (w *storageWalk) directory(root *os.Root, path string, depth int, before os.FileInfo) (err error) {
	f,err:=root.Open(path)
	if err!=nil{return err}
	defer func(){err=errors.Join(err,f.Close())}()
	opened,err:=f.Stat()
	if err!=nil{return err}
	if !opened.IsDir()||!os.SameFile(before,opened){return errors.New("storage directory changed")}
	for {
		if err:=w.ctx.Err();err!=nil{return err}
		entries,readErr:=f.ReadDir(128)
		if readErr!=nil&&readErr!=io.EOF{return readErr}
		for _,entry:=range entries {
			if err:=w.visit(root,filepath.Join(path,entry.Name()),depth+1);err!=nil{return err}
		}
		if readErr==io.EOF{break}
	}
	after,err:=f.Stat()
	if err!=nil{return err}
	if !before.ModTime().Equal(after.ModTime())||before.Size()!=after.Size(){return errors.New("storage directory changed during collection")}
	return nil
}
