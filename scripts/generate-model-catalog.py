#!/usr/bin/env python3
"""Generate the checked-in direct-download catalog from Mozilla's model registry."""

import json
from urllib.request import urlopen


REGISTRY_URL = (
    "https://storage.googleapis.com/moz-fx-translations-data--303e-prod-translations-data/"
    "db/models.json"
)

LANGUAGE_NAMES = {
    "af": "Afrikaans",
    "ar": "Arabic",
    "az": "Azerbaijani",
    "be": "Belarusian",
    "bg": "Bulgarian",
    "bn": "Bengali",
    "bs": "Bosnian",
    "ca": "Catalan",
    "cs": "Czech",
    "da": "Danish",
    "de": "German",
    "el": "Greek",
    "en": "English",
    "es": "Spanish",
    "et": "Estonian",
    "eu": "Basque",
    "fa": "Persian",
    "fi": "Finnish",
    "fr": "French",
    "gl": "Galician",
    "gu": "Gujarati",
    "hbs": "Bosnian/Croatian/Serbian",
    "he": "Hebrew",
    "hi": "Hindi",
    "hr": "Croatian",
    "hu": "Hungarian",
    "id": "Indonesian",
    "is": "Icelandic",
    "it": "Italian",
    "ja": "Japanese",
    "kn": "Kannada",
    "ko": "Korean",
    "lt": "Lithuanian",
    "lv": "Latvian",
    "ml": "Malayalam",
    "mr": "Marathi",
    "ms": "Malay",
    "nb": "Norwegian Bokmål",
    "nl": "Dutch",
    "nn": "Norwegian Nynorsk",
    "no": "Norwegian",
    "pl": "Polish",
    "pt": "Portuguese",
    "ro": "Romanian",
    "ru": "Russian",
    "sk": "Slovak",
    "sl": "Slovenian",
    "sq": "Albanian",
    "sr": "Serbian",
    "sv": "Swedish",
    "ta": "Tamil",
    "te": "Telugu",
    "th": "Thai",
    "tl": "Tagalog",
    "tr": "Turkish",
    "ug": "Uyghur",
    "uk": "Ukrainian",
    "ur": "Urdu",
    "vi": "Vietnamese",
    "zh": "Chinese",
    "zh_hant": "Traditional Chinese",
}

LANGUAGE_CONTINENTS = {
    "af": "Africa",
    "ar": "Asia",
    "az": "Asia",
    "be": "Europe",
    "bg": "Europe",
    "bn": "Asia",
    "bs": "Europe",
    "ca": "Europe",
    "cs": "Europe",
    "da": "Europe",
    "de": "Europe",
    "el": "Europe",
    "es": "Europe",
    "et": "Europe",
    "eu": "Europe",
    "fa": "Asia",
    "fi": "Europe",
    "fr": "Europe",
    "gl": "Europe",
    "gu": "Asia",
    "hbs": "Europe",
    "he": "Asia",
    "hi": "Asia",
    "hr": "Europe",
    "hu": "Europe",
    "id": "Asia",
    "is": "Europe",
    "it": "Europe",
    "ja": "Asia",
    "kn": "Asia",
    "ko": "Asia",
    "lt": "Europe",
    "lv": "Europe",
    "ml": "Asia",
    "mr": "Asia",
    "ms": "Asia",
    "nb": "Europe",
    "nl": "Europe",
    "nn": "Europe",
    "no": "Europe",
    "pl": "Europe",
    "pt": "Europe",
    "ro": "Europe",
    "ru": "Europe",
    "sk": "Europe",
    "sl": "Europe",
    "sq": "Europe",
    "sr": "Europe",
    "sv": "Europe",
    "ta": "Asia",
    "te": "Asia",
    "th": "Asia",
    "tl": "Asia",
    "tr": "Asia",
    "ug": "Asia",
    "uk": "Europe",
    "ur": "Asia",
    "vi": "Asia",
    "zh": "Asia",
    "zh_hant": "Asia",
}

CONTINENT_ORDER = (
    "Africa",
    "Asia",
    "Europe",
    "North America",
    "Oceania",
    "South America",
)


def language_pair_name(pair):
    return " → ".join(
        LANGUAGE_NAMES.get(code, code) for code in pair.split("-")
    )


def language_continent(pair):
    language = next((code for code in pair.split("-") if code != "en"), None)
    return LANGUAGE_CONTINENTS.get(language, "Other")


def format_size(size_bytes):
    if size_bytes is None:
        return "—"
    if size_bytes >= 1_000_000_000:
        return f"{size_bytes / 1_000_000_000:.1f} GB"
    return f"{size_bytes / 1_000_000:.1f} MB"


def artifact_link(base_url, artifact, label):
    path = artifact.get("path") if artifact else None
    if not path:
        return "—"
    return f"[{label}]({base_url.rstrip('/')}/{path.lstrip('/')})"


def main():
    with urlopen(REGISTRY_URL, timeout=30) as response:
        registry = json.load(response)

    models = [
        (pair, model)
        for pair, releases in registry["models"].items()
        for model in releases
    ]
    models.sort(
        key=lambda entry: (
            CONTINENT_ORDER.index(language_continent(entry[0]))
            if language_continent(entry[0]) in CONTINENT_ORDER
            else len(CONTINENT_ORDER),
            entry[0],
            entry[1].get("architecture", ""),
        )
    )

    print("# Direct model downloads")
    print()
    print(
        f"Generated from Mozilla's [model registry]({REGISTRY_URL}) "
        f"({registry['generated']}); {len(models)} model variants across "
        f"{len(registry['models'])} directed language pairs."
    )
    print(
        "Links download the registry's `.gz` artifacts. Decompress them before use; "
        "check each model's terms before redistribution. A dash means the registry "
        "does not publish that artifact or size for the variant. Weight sizes are "
        "uncompressed; vocabulary and shortlist sizes are not provided by the registry."
    )
    print(
        "Sections group each pair by the non-English language's primary geographic "
        "association; English is treated as the shared bridge language."
    )
    print()
    current_continent = None
    for pair, model in models:
        continent = language_continent(pair)
        if continent != current_continent:
            print(f"## {continent}")
            print()
            print(
                "| Language pair | Languages | Family | Status | Weight size (uncompressed) | "
                "Weights | Vocabulary | Lexical shortlist |"
            )
            print("| --- | --- | --- | --- | ---: | --- | --- | --- |")
            current_continent = continent
        files = model.get("files", {})
        print(
            "| {} | {} | {} | {} | {} | {} | {} | {} |".format(
                pair,
                language_pair_name(pair),
                model.get("architecture", "—"),
                model.get("releaseStatus") or "Unspecified",
                format_size((files.get("model") or {}).get("uncompressedSize")),
                artifact_link(registry["baseUrl"], files.get("model"), "weights"),
                artifact_link(registry["baseUrl"], files.get("vocab"), "vocabulary"),
                artifact_link(
                    registry["baseUrl"], files.get("lexicalShortlist"), "shortlist"
                ),
            )
        )


if __name__ == "__main__":
    main()