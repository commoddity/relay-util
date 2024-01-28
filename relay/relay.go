package relay

import (
	"encoding/json"
	"fmt"
)

type ID struct {
	string string
	number int

	isNumber bool
}

func IDFromString(id string) ID {
	return ID{string: id, isNumber: false}
}

func IDFromInt(id int) ID {
	return ID{number: id, isNumber: true}
}

func (i *ID) UnmarshalJSON(data []byte) error {
	var intID int
	if err := json.Unmarshal(data, &intID); err == nil {
		i.number = intID
		i.isNumber = true
		return nil
	}

	var stringID string
	if err := json.Unmarshal(data, &stringID); err == nil {
		i.string = stringID
		return nil
	}

	return fmt.Errorf("error unmarshalling ID: %s", string(data))
}

func (i ID) MarshalJSON() ([]byte, error) {
	if i.isNumber {
		return json.Marshal(i.number)
	}
	return json.Marshal(i.string)
}

func (i ID) String() string {
	if i.isNumber {
		return fmt.Sprintf("%v", i.number)
	}
	return i.string
}
