on run argv
	set outPath to item 1 of argv
	tell application "Music"
		if player state is stopped then error "stopped"
		set t to current track
		if (count of artworks of t) is 0 then error "no artwork"
		set d to raw data of artwork 1 of t
	end tell
	set f to open for access (POSIX file outPath) with write permission
	set eof f to 0
	write d to f
	close access f
end run
