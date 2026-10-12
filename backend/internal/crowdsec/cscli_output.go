package crowdsec

import (
	"bytes"
	"encoding/json"
	"errors"
)

// maxCSCLIOutput bounds the cscli output the JSON parsers accept. The largest real document, the full
// `cscli hub list -a -o json`, is well under 1 MiB.
const maxCSCLIOutput = 32 << 20

// maxJSONStartCandidates bounds how many line starts are tried as the start of the JSON document.
const maxJSONStartCandidates = 16

// errCSCLIOutput is the fixed error for output that holds no JSON document. It never carries the output.
var errCSCLIOutput = errors.New("unexpected cscli output")

// extractCSCLIJSON returns the JSON document in the combined stdout/stderr of a cscli command. cscli prints
// progress lines such as "Loaded: 168 parsers, ..." on stderr before the document, so the document is the
// first value that starts a line with '{' or '[' and decodes completely; text after it is ignored.
func extractCSCLIJSON(out []byte) ([]byte, error) {
	if len(out) > maxCSCLIOutput {
		return nil, errCSCLIOutput
	}
	tried := 0
	for i := 0; i < len(out) && tried < maxJSONStartCandidates; i++ {
		if (out[i] != '{' && out[i] != '[') || (i > 0 && out[i-1] != '\n') {
			continue
		}
		tried++
		var doc json.RawMessage
		if err := json.NewDecoder(bytes.NewReader(out[i:])).Decode(&doc); err == nil {
			return doc, nil
		}
	}
	return nil, errCSCLIOutput
}

// unmarshalCSCLIJSON decodes the JSON document in cscli output into v.
func unmarshalCSCLIJSON(out []byte, v any) error {
	doc, err := extractCSCLIJSON(out)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(doc, v); err != nil {
		return errCSCLIOutput
	}
	return nil
}
