package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"
)

var (
	proxy bproxy
	buckets []string = []string{"bucket:11.21", "bucket:22.31", "bucket:32.12"}

	upload_prefix string = "/upload/"
	delete_prefix string = "/delete/"
)

type KeyError struct {
	url string
	status int
	data []byte
}

func (k *KeyError) Error() string {
	return fmt.Sprintf("url: %s: error code: %d, returned data: '%s'", k.url, k.status, fmt.Sprintf("%s", k.data))
}

func NewKeyError(url string, status int, data []byte) error {
	return &KeyError{
		url: url,
		status: status,
		data: data,
	}
}

type bproxy struct {
	host string
	client *http.Client
}

func (p *bproxy) upload_one(url string, data []byte) (ret []byte, err error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("url: %s: new request failed: %q", url, err)
		return
	}

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("url: %s: post failed: %q", url, err)
		return
	}
	defer resp.Body.Close()

	// encode JSON
	ret, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("url: %s: readall response failed: %q", url, err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		err = NewKeyError(url, resp.StatusCode, ret)
		log.Printf("%s", err)
		return
	}

	log.Printf("%s\n", NewKeyError(url, resp.StatusCode, ret))
	return
}

func (p *bproxy) backup_key(key string) string {
	return key + ".backup"
}

func generate_url(host, key, bucket, operation string) string {
	return fmt.Sprintf("%s/%s/%s?bucket=%s", host, operation, key, bucket)
}

func (p *bproxy) generate_url(key, bucket, operation string) string {
	return generate_url(p.host, key, bucket, operation)
}

func (p *bproxy) remove_one(key, bucket string) (ret []byte, err error) {
	url := p.generate_url(key, bucket, "delete")

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Printf("url: %s: new request failed: %q", url, err)
		return
	}

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("url: %s: post failed: %q", url, err)
		return
	}
	defer resp.Body.Close()

	// encode JSON
	ret, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("url: %s: readall response failed: %q", url, err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		err = NewKeyError(url, resp.StatusCode, ret)
		log.Printf("%s", err)
		return
	}

	log.Printf("%s\n", NewKeyError(url, resp.StatusCode, ret))
	return
}

func upload_handler(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("url: %s: readall failed: %q", r.URL, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	bucket := buckets[rand.Intn(len(buckets))]
	key := r.URL.Path[len(upload_prefix):]
	url := proxy.generate_url(key, bucket, "upload")

	ret_primary, err := proxy.upload_one(url, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	backup_key := proxy.backup_key(key)
	backup_url := proxy.generate_url(backup_key, bucket, "upload")
	ret_backup, err := proxy.upload_one(backup_url, data)
	if err != nil {
		_, _ = proxy.remove_one(key, bucket)

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	type ent_reply struct {
		Get string `json:"get"`
		Update string `json:"update"`
		Delete string `json:"delete"`
		Key string `json:"key"`
		Reply string `json:"reply"`
	}
	type upload_reply struct {
		Bucket string `json:"bucket"`
		Primary ent_reply `json:"primary"`
		Backup ent_reply `json:"backup"`
	}

	reply := upload_reply{
		Bucket: bucket,
		Primary: ent_reply{
			Key: key,
			Get: proxy.generate_url(key, bucket, "get"),
			Update: url,
			Delete: generate_url(r.Host, key, bucket, "delete"),
			Reply: string(ret_primary),
		},
		Backup: ent_reply{
			Key: backup_key,
			Get: proxy.generate_url(backup_key, bucket, "get"),
			Reply: string(ret_backup),
		},
	}

	reply_json, err := json.Marshal(reply)
	if err != nil {
		proxy.remove_one(key, bucket)
		proxy.remove_one(backup_key, bucket)

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Error(w, string(reply_json), http.StatusOK)
}

func delete_handler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[len(delete_prefix):]

	q := r.URL.Query()
	bucket := q.Get("bucket")
	if len(bucket) == 0 {
		err := NewKeyError(r.URL.String(), http.StatusBadRequest, []byte("there is no 'bucket' URI parameter"))
		log.Printf("%s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proxy.remove_one(key, bucket)

	backup_key := proxy.backup_key(key)
	backup_url := proxy.generate_url(backup_key, bucket, "update")

	type update struct {
		Id string `json:"id"`
		Indexes map[string]string `json:"indexes"`
	}

	del_time := time.Now().Add(2 * 24 * 3600 * time.Second).Unix()

	update_json := update{
		Id: backup_key,
		Indexes: map[string]string {
			"delete" : fmt.Sprintf("%s", del_time),
		},
	}

	update_bytes, err := json.Marshal(update_json)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ret, err := proxy.upload_one(backup_url, update_bytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Error(w, string(ret), http.StatusOK)
}

func main() {
	rand.Seed(9)

	proxy.client = &http.Client{}
	proxy.host = "http://108.61.155.67:80"

	http.HandleFunc(upload_prefix, upload_handler)
	http.HandleFunc(delete_prefix, delete_handler)

	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
