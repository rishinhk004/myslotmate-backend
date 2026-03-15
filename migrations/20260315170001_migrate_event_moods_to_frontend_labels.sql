-- Migrate legacy event mood values to the frontend-aligned labels.

UPDATE events SET mood = 'adventurous' WHERE mood = 'adventure';
UPDATE events SET mood = 'relaxing' WHERE mood = 'chill';
UPDATE events SET mood = 'creative' WHERE mood = 'romantic';
UPDATE events SET mood = 'educational' WHERE mood = 'intellectual';
UPDATE events SET mood = 'culinary' WHERE mood = 'foodie';
UPDATE events SET mood = 'cultural' WHERE mood = 'nightlife';
