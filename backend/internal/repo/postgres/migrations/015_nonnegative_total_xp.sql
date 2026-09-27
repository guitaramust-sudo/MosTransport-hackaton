UPDATE players SET total_xp = 0 WHERE total_xp < 0;
ALTER TABLE players ADD CONSTRAINT players_total_xp_nonnegative CHECK (total_xp >= 0);
