package daemon

import "strconv"

func subjectProbe(pid int, start uint64, uid uint32) string {
	return strconv.Itoa(pid) + "," + strconv.FormatUint(start, 10) + "," + strconv.FormatUint(uint64(uid), 10)
}
