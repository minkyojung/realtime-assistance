// Music.app 라이브러리를 통째로 JSON 으로 내보낸다.
//
// 속성마다 이벤트를 한 번씩만 보낸다. 곡마다 부르면 이벤트가 곡 수만큼 늘어나
// 수천 곡에서 분 단위가 된다. 목록으로 한 번에 받으면 곡 수와 거의 무관하다.
// (실측 157곡 0.16초 — docs/05)
//
// 구분자를 쓰지 않는 이유: 제목에 `|` 도 줄바꿈도 들어간다.
// JSON 으로 내보내면 이스케이프를 엔진이 대신 해준다.
function run() {
  try {
    const M = Application('Music');
    const out = { tracks: [], playlists: [] };
    const seen = {}; // persistentID -> 이미 담았는지

    // 속성 하나가 막혀도 덤프 전체를 버리지 않는다.
    // 길이가 어긋나면 그 속성만 낮춘다 — 필드가 밀리면 안 된다.
    const bulk = (fn, n) => {
      try {
        const v = fn();
        return v && v.length === n ? v : new Array(n).fill(null);
      } catch (e) {
        return new Array(n).fill(null);
      }
    };

    // missing value 는 null 로 온다. 1970 이전은 없는 값으로 본다.
    const iso = (d) =>
      d && d.getTime && d.getTime() > 0 ? new Date(d.getTime()).toISOString() : null;

    // col 은 tracks 컬렉션. 돌려주는 것은 그 순서 그대로의 persistentID 목록이다.
    const take = (col, inLibrary) => {
      const pid = col.persistentID();
      const n = pid.length;
      if (n === 0) return [];
      const name = bulk(() => col.name(), n);
      const artist = bulk(() => col.artist(), n);
      const album = bulk(() => col.album(), n);
      const aArt = bulk(() => col.albumArtist(), n);
      const genre = bulk(() => col.genre(), n);
      const year = bulk(() => col.year(), n);
      const dur = bulk(() => col.duration(), n);
      const plays = bulk(() => col.playedCount(), n);
      const skips = bulk(() => col.skippedCount(), n);
      const pDate = bulk(() => col.playedDate(), n);
      const aDate = bulk(() => col.dateAdded(), n);
      const fav = bulk(() => col.favorited(), n);
      const dis = bulk(() => col.disliked(), n);
      const cls = bulk(() => col.class(), n);

      for (let i = 0; i < n; i++) {
        if (!pid[i] || seen[pid[i]]) continue;
        seen[pid[i]] = true;
        out.tracks.push({
          persistentId: pid[i],
          title: name[i] || '',
          artist: artist[i] || '',
          albumTitle: album[i] || '',
          albumArtist: aArt[i] || artist[i] || '',
          genre: genre[i] || null,
          year: year[i] > 0 ? year[i] : null,
          durationMs: Math.round((dur[i] || 0) * 1000),
          playCount: plays[i] || 0,
          skipCount: skips[i] || 0,
          favorited: fav[i] === true,
          disliked: dis[i] === true,
          source: cls[i] === 'fileTrack' ? 'file' : 'shared',
          inLibrary: inLibrary,
          // 담은 적이 없으면 담은 날짜도 없다. 지어내지 않는다 (docs/05 3-4).
          addedAt: inLibrary ? iso(aDate[i]) : null,
          lastPlayedAt: iso(pDate[i]),
        });
      }
      return pid;
    };

    take(M.libraryPlaylists[0].tracks, true);

    // 라이브러리에 없고 플레이리스트에만 담긴 곡이 실제로 있다 (docs/05 3-3).
    const pls = M.userPlaylists;
    // 개수를 미리 모르므로 bulk 를 못 쓴다. 이름을 먼저 받아 그 길이를 기준으로 삼는다.
    let plName = [];
    try {
      plName = pls.name() || [];
    } catch (e) {
      plName = [];
    }
    const plKind = bulk(() => pls.specialKind(), plName.length);
    const plPid = bulk(() => pls.persistentID(), plName.length);
    for (let i = 0; i < plName.length; i++) {
      // "Music" 같은 특수 목록은 라이브러리를 통째로 다시 담는다.
      if (plKind[i] && plKind[i] !== 'none') continue;
      let ids;
      try {
        ids = pls[i].tracks.persistentID();
      } catch (e) {
        continue;
      }
      // 새 곡이 있을 때만 나머지 속성까지 읽는다.
      // 대부분은 라이브러리의 부분집합이라 이벤트 한 번으로 끝난다.
      if (ids.some((p) => !seen[p])) ids = take(pls[i].tracks, false);
      out.playlists.push({ persistentId: plPid[i], name: plName[i], trackIds: ids });
    }
    return JSON.stringify(out);
  } catch (e) {
    return JSON.stringify({ error: String(e) });
  }
}
