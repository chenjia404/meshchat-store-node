package pebble

import (
	"fmt"
	"strconv"
	"strings"
)

func msgPrefix(recipientID string) string {
	return "msg/" + recipientID + "/"
}

func msgKey(recipientID string, storeSeq uint64) string {
	return fmt.Sprintf("%s%020d", msgPrefix(recipientID), storeSeq)
}

func dupKey(recipientID, msgID string) string {
	return "dup/" + recipientID + "/" + msgID
}

func seqKey(recipientID string) string {
	return "seq/" + recipientID
}

func expKey(expireAt int64, recipientID string, storeSeq uint64) string {
	return fmt.Sprintf("exp/%020d/%s/%020d", expireAt, recipientID, storeSeq)
}

func deliveredKey(recipientID string) string {
	return "state/" + recipientID + "/delivered"
}

func ackedKey(recipientID string) string {
	return "state/" + recipientID + "/acked"
}

func expUpperBound(now int64) string {
	return fmt.Sprintf("exp/%020d/", now+1)
}

func prefixUpperBound(prefix string) []byte {
	out := append([]byte{}, []byte(prefix)...)
	return append(out, 0xFF)
}

func parseUint64(data []byte) (uint64, error) {
	return strconv.ParseUint(string(data), 10, 64)
}

func parseMsgKey(key []byte) (recipientID string, storeSeq uint64, err error) {
	parts := strings.Split(string(key), "/")
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("invalid msg key: %s", string(key))
	}
	storeSeq, err = strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return parts[1], storeSeq, nil
}

func parseExpKey(key []byte) (expireAt int64, recipientID string, storeSeq uint64, err error) {
	parts := strings.Split(string(key), "/")
	if len(parts) != 4 {
		return 0, "", 0, fmt.Errorf("invalid exp key: %s", string(key))
	}
	expireAt, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, "", 0, err
	}
	storeSeq, err = strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		return 0, "", 0, err
	}
	return expireAt, parts[2], storeSeq, nil
}
