package store

import (
	"asdf/internal/resource"
	"asdf/internal/types"
	"encoding/json"
	"errors"
	"log"
	"os"
)

type Data struct {
	data []types.JRD
}

func NewData() *Data {
	return &Data{}
}

func (app *Data) LoadData(fileName string) error {
	dir, _ := os.Getwd()
	file, err := os.Open(fileName)
	if err != nil {
		log.Printf("Error opening file: %s", err.Error()+dir)
		return errors.New("Error loading file")
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&app.data); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		return errors.New("Error decoding JSON")
	}
	return nil
}

func (app *Data) LookupResource(subject string) (*types.JRD, error) {
	for _, jrd := range app.data {
		acct, err := resource.GetSubject(jrd.Subject)
		if err != nil {
			return nil, err
		}
		if acct == subject {
			return &jrd, nil
		}
	}
	return nil, nil
}

func (app *Data) SaveData(fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		log.Printf("Error creating file: %v", err)
		return errors.New("Error creating file")
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(app.data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
		return errors.New("Error encoding JSON")
	}
	return nil
}
