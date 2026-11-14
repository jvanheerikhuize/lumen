import { test } from 'node:test';
import assert from 'node:assert/strict';
import { formatLang, matchesLanguage, normalizeSubtitleLang } from '../src/lib/subtitleLanguages.ts';

test('matches 2-letter and 3-letter OpenSubtitles language codes', () => {
  assert.equal(matchesLanguage('eng', 'en'), true);
  assert.equal(matchesLanguage('nld', 'nl'), true);
  assert.equal(matchesLanguage('dut', 'nl'), true);
  assert.equal(matchesLanguage('pob', 'pt'), true);
  assert.equal(matchesLanguage('eng', 'off'), false);
  assert.equal(matchesLanguage('fra', 'en'), false);
});

test('normalizes 3-letter OpenSubtitles language codes to 2-letter settings codes', () => {
  assert.equal(normalizeSubtitleLang('eng'), 'en');
  assert.equal(normalizeSubtitleLang('dut'), 'nl');
  assert.equal(normalizeSubtitleLang('jpn'), 'ja');
  assert.equal(normalizeSubtitleLang('off'), 'off');
});

test('formats special and standard subtitle language labels', () => {
  assert.equal(formatLang('pob'), 'Portuguese (Brazil)');
  assert.equal(formatLang('zht'), 'Chinese (Traditional)');
  assert.equal(formatLang('eng'), 'English');
});
