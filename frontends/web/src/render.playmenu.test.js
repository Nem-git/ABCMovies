import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';

import { GetPlayInfoResponseSchema } from './gen/abcmovies/api/v1/core_pb.js';
import { playMenu, playTracks } from './render.js';

// GetPlayInfo's menu is a oneof (PLAN.md §6.1): the raw branch lists one relay
// token per located track; the packaged branch carries a manifest URL and no
// per-track entries. The player reads tracks from the raw branch — reading a
// flat `tracks` field instead silently yields an empty menu.
describe('play menu reads the raw per-track branch', () => {
  const raw = create(GetPlayInfoResponseSchema, {
    menu: {
      case: 'rawTracks',
      value: {
        tracks: [
          {
            trackId: 'container',
            media: { case: 'video', value: { codec: 'h264' } },
            relayUrl: '/media/relay/container-token',
          },
        ],
      },
    },
    container: 'mkv',
  });

  it('playTracks returns the raw branch tracks', () => {
    expect(playTracks(raw).map((t) => t.trackId)).toEqual(['container']);
  });

  it('playTracks yields nothing for the packaged branch or a missing menu', () => {
    const packaged = create(GetPlayInfoResponseSchema, {
      menu: { case: 'manifestUrl', value: '/media/relay/hls/token' },
    });
    expect(playTracks(packaged)).toEqual([]);
    expect(playTracks(undefined)).toEqual([]);
    expect(playTracks({ tracks: [{ trackId: 'x' }] })).toEqual([]);
  });

  it('playMenu renders one .track per located track', () => {
    const body = playMenu(raw);
    expect(body.querySelectorAll('.track')).toHaveLength(1);
    expect(body.querySelector('.track code')?.textContent).toBe('container');
  });

  it('playMenu renders the waiting hint when no track is located', () => {
    const body = playMenu(
      create(GetPlayInfoResponseSchema, {
        menu: { case: 'manifestUrl', value: '/media/relay/hls/token' },
      }),
    );
    expect(body.querySelectorAll('.track')).toHaveLength(0);
    expect(body.querySelector('.empty')).not.toBeNull();
  });
});
