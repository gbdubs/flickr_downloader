# Flickr Downloader

This CLI downloads content from Flickr, writes attribution EXIF data, and can be accessed through the command line or through Go.

## Functionality 

1) You can request to download multiple photos that match your query, and the CLI will download them in parallel.
2) Attribution information (License, Authorship, Link to original) are written into the EXIF metadata of the results.
3) Doesn't return any two results from the same creator, avoiding duplicates and near-dupes.
4) Searches by license, preferring the most lenient Creative Commons licenses.
5) Results are stored as JPEGs, with standard [EXIF metadata](https://www.media.mit.edu/pia/Research/deepview/exif.html).

## Usage

You'll first need to request a [Flickr API key](https://www.flickr.com/services/apps/create/apply/) (it's easy!).

Then, place this value in the `const` declaration in the file called `API_KEY` (you'll get a warning if you forget). Then it's just:

```
go build; ./flickr_downloader --query="Trash Panda" --output="NewDirForOutput" --number_of_images=10
```

for flag documentation, run

```
./flickr_downloader --help
```

## Support

This is not a project I actively support. It was written for a series of side projects and is unlikely to be of much use to others. If you want to add any functionality, I wholeheartedly welcome it.
