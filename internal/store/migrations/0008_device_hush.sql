-- How a speaker is kept quiet while Xiao AI answers: '' (up to the model) | normal | strong | wait.
ALTER TABLE devices ADD COLUMN hush TEXT NOT NULL DEFAULT '';
