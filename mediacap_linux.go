package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

type MediaSessionProvider struct {
	lastMetadata AudioMetadata
	conn         *dbus.Conn
	lastCheck    time.Time
}

func NewMediaSessionProvider() (*MediaSessionProvider, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	return &MediaSessionProvider{
		conn:         conn,
		lastMetadata: DefaultMetadata(),
		lastCheck:    time.Now(),
	}, nil
}

func (msp *MediaSessionProvider) Close() error {
	if msp.conn != nil {
		return msp.conn.Close()
	}
	return nil
}

func (msp *MediaSessionProvider) GetCurrentMedia() AudioMetadata {
	if time.Since(msp.lastCheck) < 2*time.Second {
		return msp.lastMetadata
	}
	msp.lastCheck = time.Now()

	var names []string
	if err := msp.conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		log.Printf("Failed to list D-Bus names: %v", err)
		return msp.lastMetadata
	}

	var players []string
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			players = append(players, name)
		}
	}

	if len(players) == 0 {
		return msp.lastMetadata
	}

	for _, playerName := range players {
		metadata := msp.queryPlayer(playerName)
		if metadata.IsPlaying {
			msp.lastMetadata = metadata
			return metadata
		}
	}

	for _, playerName := range players {
		metadata := msp.queryPlayer(playerName)
		if metadata.AppName != "Unknown" {
			msp.lastMetadata = metadata
			return metadata
		}
	}

	return msp.lastMetadata
}

func (msp *MediaSessionProvider) queryPlayer(busName string) AudioMetadata {
	obj := msp.conn.Object(busName, "/org/mpris/MediaPlayer2")

	var status string
	if err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.mpris.MediaPlayer2.Player", "PlaybackStatus").Store(&status); err != nil {
		return DefaultMetadata()
	}

	isPlaying := (status == "Playing")

	var metadataVariant dbus.Variant
	if err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.mpris.MediaPlayer2.Player", "Metadata").Store(&metadataVariant); err != nil {
		return AudioMetadata{AppName: extractAppName(busName), IsPlaying: isPlaying}
	}

	metadataMap, ok := metadataVariant.Value().(map[string]dbus.Variant)
	if !ok {
		return AudioMetadata{AppName: extractAppName(busName), IsPlaying: isPlaying}
	}

	artist := extractStringArray(metadataMap, "xesam:artist")
	album := extractString(metadataMap, "xesam:album")
	title := extractString(metadataMap, "xesam:title")
	if artist == "" && album != "" {
		artist = album
	}

	return AudioMetadata{
		AppName:    extractAppName(busName),
		ArtistName: artist,
		SongName:   title,
		IsPlaying:  isPlaying,
	}
}
