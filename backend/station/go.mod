module avtotest.uz/station

// Pinned to 1.20 because Go 1.21 dropped Windows 7 / 8 / Server 2008 / 2012,
// and classroom PCs in driving schools are still on Windows 7 -- a binary
// built with a newer toolchain refuses to start there. Its output runs fine on
// Windows 10 and 11 too, so one build covers the whole fleet. Only this module
// is pinned; the server stays on the current toolchain.
go 1.20

require (
	github.com/tc-hib/winres v0.2.1
	golang.org/x/sys v0.15.0
)

require (
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	golang.org/x/image v0.12.0 // indirect
)
