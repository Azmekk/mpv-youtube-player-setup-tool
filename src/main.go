package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/manifoldco/promptui"
)

const (
	ytdlpEnum      = 1
	mpvEnum        = 2
	ytdlpLocalPath = "mpv/yt-dlp.exe"
	mpvLocalPath   = "mpv/mpv.exe"

	ytdlpDownloadUrl = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	mpvDownloadUrl   = "https://nightly.link/mpv-player/mpv/workflows/build/master/mpv-x86_64-windows-msvc.zip"
)

func extractMpv() error {
	fmt.Println("Extracting mpv, please wait...")
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	exeDir := filepath.Dir(exePath)
	mpvExtractDir := filepath.Join(exeDir, "mpv")
	mpvZipPath := filepath.Join(exeDir, "mpv", "mpv.zip")

	archive, err := zip.OpenReader(mpvZipPath)
	if err != nil {
		return err
	}

	for _, f := range archive.File {
		filePath := filepath.Join(mpvExtractDir, f.Name)

		if !strings.HasPrefix(filePath, filepath.Clean(mpvExtractDir)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal file path", filePath)
		}

		if f.FileInfo().Name() == "mpv.com" {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}
		defer fileInArchive.Close()

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	archive.Close()

	if err := os.Remove(mpvZipPath); err != nil {
		return err
	}

	return nil
}

func getOrInstallDependencies() error {

	exePath, err := os.Executable()
	exeDir := filepath.Dir(exePath)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get current executable path")
		return err
	}

	wg := sync.WaitGroup{}
	ytDLpPath := filepath.Join(exeDir, ytdlpLocalPath)

	if _, err := os.Stat(ytDLpPath); err == nil {
		os.Setenv("PATH", fmt.Sprintf("%s;%s", filepath.Dir(ytDLpPath), os.Getenv("PATH")))
	} else if _, err := exec.LookPath("yt-dlp.exe"); err != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadDependency(ytdlpEnum)
			os.Setenv("PATH", fmt.Sprintf("%s;%s", filepath.Dir(ytDLpPath), os.Getenv("PATH")))
		}()
	}

	mpvLocalPath := filepath.Join(exeDir, mpvLocalPath)

	if _, err := os.Stat(mpvLocalPath); err == nil {
		os.Setenv("PATH", fmt.Sprintf("%s;%s", filepath.Dir(mpvLocalPath), os.Getenv("PATH")))
	} else if _, err := exec.LookPath("mpv.exe"); err != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadDependency(mpvEnum)
			err := extractMpv()

			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to extract mpv: %s\n", err)
				return
			}

			os.Setenv("PATH", fmt.Sprintf("%s;%s", filepath.Dir(mpvLocalPath), os.Getenv("PATH")))
		}()
	}

	wg.Wait()
	return nil
}

func downloadFile(url string, output string) {
	err := os.MkdirAll(filepath.Dir(output), os.ModePerm)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to create directories")
		return
	}

	out, err := os.Create(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to create file")
		return
	}

	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get file")
		return
	}

	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to write file")
		return
	}

	fmt.Printf("Finished downloading %s\n", filepath.Base(output))
}

func downloadDependency(dependency int) {
	exePath, err := os.Executable()

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get current executable path")
		return
	}

	exeDir := filepath.Dir(exePath)

	switch dependency {
	case ytdlpEnum:

		fmt.Println("Downloading yt-dlp, please wait...")
		downloadFile(ytdlpDownloadUrl, ytdlpLocalPath)

	case mpvEnum:
		mpvDir := filepath.Join(exeDir, "mpv")
		mpvZip := filepath.Join(mpvDir, "mpv.zip")

		fmt.Println("Downloading mpv, please wait...")
		downloadFile(mpvDownloadUrl, mpvZip)
	}
}

func clearConsole() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func main() {
	getOrInstallDependencies()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter video URL: ")
	url, _ := reader.ReadString('\n')

	qualityPrompt := promptui.Select{
		Label: "Select quality to aim for",
		Items: []string{"2160p", "1444p", "1080p", "720p", "480p", "360p", "240p", "144p"},
	}

	_, promptResult, err := qualityPrompt.Run()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Prompt failed %v\n", err)
		return
	}

	hwDecPrompt := promptui.Select{
		Label: "Use hardware decoding?",
		Items: []string{"Yes (Default)", "No (Only use in case of issues with playback)"},
	}

	_, hwDecPromptResult, err := hwDecPrompt.Run()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Prompt failed %v\n", err)
		return
	}
	clearConsole()

	trimmedPromptResult := strings.TrimSuffix(promptResult, "p")
	formatArg := fmt.Sprintf("--ytdl-format=bestvideo[height<=%s]+bestaudio/best[height<=%s]", trimmedPromptResult, trimmedPromptResult)

	mpvArgs := []string{formatArg}

	if strings.HasPrefix(hwDecPromptResult, "Yes") {
		mpvArgs = append(mpvArgs, "--hwdec=auto")
		fmt.Println("Starting video playback with hardware decoding on. This might take a second...")
	} else {
		fmt.Println("Starting video playback with hardware decoding off. This might take a second...")
	}

	mpvArgs = append(mpvArgs, url)

	cmd := exec.Command("mpv.exe", mpvArgs...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cmd.Run() failed with %s\n", err)
	}
}
