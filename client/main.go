package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	galaxyServerAddress = "http://8.140.162.132/"
	loginHeaderApi      = "login/header"
)

func main() {
	myUserId := generateRandomSteamUserId()
	fmt.Println("Generated random UserId:", myUserId)

	fullUrl, err := getFullDownloadRequestUrl(myUserId)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Latest url:", fullUrl)
	// fullUrl := "20230718173125"

	if !fileExists(fullUrl) {
		fmt.Printf("New version of full-data found, downloading %v...\n", fullUrl)
		if err := downloadFullData(fullUrl); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Printf("Full-data %v is up-to-date\n", fullUrl)
	}
	fmt.Printf("Parsing %v into tables...\n", fullUrl)
	parseFullData(fullUrl)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func generateRandomSteamUserId() uint64 {
	return 1 | (1 << 32) | (1 << 52) | (1 << 56) | (uint64(rand.Int31()) << 1)
}

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	return io.ReadAll(resp.Body)
}

func getFullDownloadRequestUrl(myUserId uint64) (string, error) {
	data, err := httpGet(galaxyServerAddress + loginHeaderApi + "?user_id=" + fmt.Sprint(myUserId))
	if err != nil {
		return "", err
	}
	splitted := strings.Split(string(data), ",")
	if len(splitted) != 2 {
		return "", fmt.Errorf("invalid response: %v", string(data))
	}
	return splitted[1], nil
}

func downloadFullData(url string) error {
	var data []byte
	var err error
	if data, err = httpGet(galaxyServerAddress + "download/" + url); err != nil {
		return err
	}
	var zr *gzip.Reader
	buf := bytes.NewReader(data)
	if zr, err = gzip.NewReader(buf); err != nil {
		return err
	}
	var of *os.File
	if of, err = os.Create(url); err != nil {
		return err
	}
	defer func() {
		_ = of.Close()
	}()
	if _, err = io.Copy(of, zr); err != nil {
		return err
	}
	return nil
}

func parseFullData(filename string) error {
	var data []byte
	var err error
	if data, err = os.ReadFile(filename); err != nil {
		return err
	}
	r := bytes.NewReader(data)
	var v32 uint32
	if err = binary.Read(r, binary.LittleEndian, &v32); err != nil {
		return err
	}
	if err = loadTopTenPlayerData(r); err != nil {
		return err
	}
	if err = loadOtherData(r); err != nil {
		return err
	}
	return nil
}

func loadTopTenPlayerData(reader io.Reader) error {
	var v32 uint32
	var err error
	if err = binary.Read(reader, binary.LittleEndian, &v32); err != nil {
		return err
	}
	var num int32
	if err = binary.Read(reader, binary.LittleEndian, &num); err != nil {
		return err
	}
	var of *os.File
	if of, err = os.Create("top_ten.csv"); err != nil {
		return err
	}
	defer func() {
		_ = of.Close()
	}()
	w := csv.NewWriter(of)
	defer w.Flush()
	if err = w.Write([]string{"种子", "星数", "资源倍率", "用户ID", "平台", "账号", "发电量", "匿名"}); err != nil {
		return err
	}
	for i := int32(0); i < num; i++ {
		var seedKey, userId int64
		var platform byte
		var name string
		var genCap int64
		var isAnon byte
		if err := binary.Read(reader, binary.LittleEndian, &seedKey); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &userId); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &platform); err != nil {
			return err
		}
		if nameLen, err := read7BitEncodedInt(reader); err != nil {
			return err
		} else {
			nameBytes := make([]byte, nameLen)
			if err := binary.Read(reader, binary.LittleEndian, nameBytes); err != nil {
				return err
			}
			name = string(nameBytes)
		}
		if err := binary.Read(reader, binary.LittleEndian, &genCap); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &isAnon); err != nil {
			return err
		}
		if err = w.Write([]string{fmt.Sprint(seedKey / 100000000), fmt.Sprint((seedKey / 100000) % 1000), resourceMultiplier((seedKey / 1000) % 100), fmt.Sprint(userId), platformName(platform), name, fmt.Sprint(genCap * 60), fmt.Sprint(isAnon > 0)}); err != nil {
			return err
		}
	}
	return nil
}

func loadOtherData(reader io.Reader) error {
	var err error
	var v32 uint32
	{
		if err = binary.Read(reader, binary.LittleEndian, &v32); err != nil {
			return err
		}
		var totalGenCap, totalSailLaunched int64
		var totalPlayer, totalDysonSphere int32
		if err = binary.Read(reader, binary.LittleEndian, &totalGenCap); err != nil {
			return err
		}
		if err = binary.Read(reader, binary.LittleEndian, &totalSailLaunched); err != nil {
			return err
		}
		if err = binary.Read(reader, binary.LittleEndian, &totalPlayer); err != nil {
			return err
		}
		if err = binary.Read(reader, binary.LittleEndian, &totalDysonSphere); err != nil {
			return err
		}
		var of *os.File
		if of, err = os.Create("summary.txt"); err != nil {
			return err
		}
		defer func() {
			_ = of.Close()
		}()
		if _, err = fmt.Fprintf(of, "总玩家数: %v\n总发电量: %v\n总太阳帆数: %v\n总戴森球数: %v\n", totalPlayer, totalGenCap*60, totalSailLaunched, totalDysonSphere); err != nil {
			return err
		}
	}
	{
		if err = binary.Read(reader, binary.LittleEndian, &v32); err != nil {
			return err
		}
		var of *os.File
		if of, err = os.Create("all.csv"); err != nil {
			return err
		}
		defer func() {
			_ = of.Close()
		}()
		w := csv.NewWriter(of)
		defer w.Flush()
		if err = w.Write([]string{"种子", "星数", "资源倍率", "用户数", "总发电量"}); err != nil {
			return err
		}
		var num int32
		if err = binary.Read(reader, binary.LittleEndian, &num); err != nil {
			return err
		}
		for i := int32(0); i < num; i++ {
			var seedKey int64
			var genCap float32
			var playerNum int32
			if err := binary.Read(reader, binary.LittleEndian, &seedKey); err != nil {
				return err
			}
			if err := binary.Read(reader, binary.LittleEndian, &genCap); err != nil {
				return err
			}
			if err := binary.Read(reader, binary.LittleEndian, &playerNum); err != nil {
				return err
			}
			if err = w.Write([]string{fmt.Sprint(seedKey / 100000000), fmt.Sprint((seedKey / 100000) % 1000), resourceMultiplier((seedKey / 1000) % 100), fmt.Sprint(playerNum), fmt.Sprint(int64(genCap * 60))}); err != nil {
				return err
			}
			if err = binary.Read(reader, binary.LittleEndian, &v32); err != nil {
				return err
			}
		}
	}
	return nil
}

func platformName(id byte) string {
	switch id {
	case 1:
		return "Steam"
	case 2:
		return "WeGame"
	case 3:
		return "XGP"
	default:
		return "Standalone"
	}
}

func resourceMultiplier(n int64) string {
	if n == 99 {
		return "无限"
	}
	return strconv.FormatFloat(float64(n)/10.0, 'f', 1, 64)
}

func read7BitEncodedInt(reader io.Reader) (int32, error) {
	var num int32 = 0
	var num2 int32 = 0
	for num2 != 35 {
		var b byte
		if err := binary.Read(reader, binary.LittleEndian, &b); err != nil {
			return -1, err
		}
		num |= int32(b&127) << num2
		num2 += 7
		if (b & 128) == 0 {
			return num, nil
		}
	}
	return -1, errors.New("too many bytes in what should have been a 7 bit encoded Int32")
}
