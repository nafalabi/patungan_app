/** @type {import('tailwindcss').Config} */
module.exports = {
	content: [
		"./web/templates/**/*.html", 
		"./web/templates/**/*.templ",
		"./internal/**/*.templ",
		"./internal/**/*.go"
	],
	theme: {
		extend: {
			colors: {
				primary: {
					DEFAULT: '#18181b', // Zinc 900
					hover: '#27272a',   // Zinc 800
				},
				bg: {
					body: '#fafafa',    // Zinc 50
					card: '#ffffff',    // Clean white
					hover: '#f4f4f5',   // Zinc 100
					subtle: '#f4f4f5',  // Zinc 100
				},
				text: {
					primary: '#18181b', // Zinc 900
					secondary: '#71717a', // Zinc 500
					muted: '#a1a1aa',   // Zinc 400
				},
				border: {
					DEFAULT: '#e4e4e7', // Zinc 200
					subtle: '#f4f4f5',  // Zinc 100
				},
				success: {
					DEFAULT: '#166534', // Emerald 800
					bg: '#f0fdf4',      // Emerald 50
					border: '#dcfce7',  // Emerald 100
				},
				warning: {
					DEFAULT: '#854d0e', // Yellow 800
					bg: '#fefce8',      // Yellow 50
					border: '#fef08a',  // Yellow 200
				},
				danger: {
					DEFAULT: '#9f1239', // Rose 800
					bg: '#fff1f2',      // Rose 50
					border: '#ffe4e6',  // Rose 200
				},
				input: {
					bg: '#ffffff'
				}
			},
			boxShadow: {
				'xs': '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
				'card': '0 1px 3px 0 rgba(0, 0, 0, 0.03), 0 1px 2px -1px rgba(0, 0, 0, 0.03)',
				'card-hover': '0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.05)',
				'popover': '0 10px 25px -5px rgba(0, 0, 0, 0.08), 0 8px 10px -6px rgba(0, 0, 0, 0.08)',
			},
			backdropBlur: {
				'xs': '2px',
			},
			fontFamily: {
				sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
			},
			keyframes: {
				fadeIn: {
					'from': { opacity: '0', transform: 'translateY(20px)' },
					'to': { opacity: '1', transform: 'translateY(0)' },
				}
			},
			animation: {
				fadeIn: 'fadeIn 0.5s ease-out',
			}
		},
	},
	plugins: [],
}
