# Сторонние компоненты

`ttd` распространяется по лицензии MIT (см. [LICENSE](LICENSE)). В бинарник
встроены Python-пакеты, которые ставятся в отдельное окружение и запускаются
как внешние процессы. Они не являются частью исходного кода `ttd` и
распространяются на условиях собственных лицензий.

## Встроенные движки

| Пакет | Версия | Лицензия | Источник |
| --- | --- | --- | --- |
| [gallery-dl](https://github.com/mikf/gallery-dl) | 1.32.17 | GPL-2.0-only | <https://pypi.org/project/gallery-dl/> |
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | 2026.8.19 | Unlicense | <https://pypi.org/project/yt-dlp/> |

## Зависимости gallery-dl

| Пакет | Версия | Лицензия | Источник |
| --- | --- | --- | --- |
| [requests](https://github.com/psf/requests) | 2.34.2 | Apache-2.0 | <https://pypi.org/project/requests/> |
| [urllib3](https://github.com/urllib3/urllib3) | 2.8.0 | MIT | <https://pypi.org/project/urllib3/> |
| [certifi](https://github.com/certifi/python-certifi) | 2026.10.19 | MPL-2.0 | <https://pypi.org/project/certifi/> |
| [idna](https://github.com/kjd/idna) | 3.20 | BSD-3-Clause | <https://pypi.org/project/idna/> |
| [charset-normalizer](https://github.com/jawah/charset_normalizer) | 3.5.2 | MIT | <https://pypi.org/project/charset-normalizer/> |

## Где взять полные тексты лицензий

Колёса лежат в каталоге [`internal/installer/wheels`](internal/installer/wheels).
Каждое колесо содержит свои файлы лицензий в каталоге
`<пакет>-<версия>.dist-info/licenses/` — их можно посмотреть, распаковав `.whl`
как обычный zip-архив.

## Обновление

Встроенные версии зафиксированы на момент сборки. Обновить движки можно
командой `ttd install --online` (с PyPI) или `ttd install --wheels <каталог>`
из заранее скачанных колёс. При обновлении действуют лицензии новых версий.
