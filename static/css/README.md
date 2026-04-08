# Local Static Assets

Place downloaded CDN assets here for offline/local use:

## Required files

1. `tailwind.css` - Pre-built Tailwind CSS (run: `npx tailwindcss -o static/css/tailwind.css --minify`)
2. `fontawesome.min.css` - Font Awesome 6.5.1 CSS
3. `inter.css` - Inter font CSS with local font files in ../fonts/

## How to generate

```bash
# Tailwind (requires Node.js)
npx tailwindcss -i /dev/null -o static/css/tailwind.css --minify

# Font Awesome
curl -o static/css/fontawesome.min.css https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css

# Inter font
curl -o static/css/inter.css "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap"
```

When these files exist, the layout template will prefer them over CDN.
