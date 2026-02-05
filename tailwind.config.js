/** @type {import('tailwindcss').Config} */
module.exports = {
	content: ["./web/templates/**/*.html"],
	theme: {
		extend: {
			colors: {
				primary: {
					DEFAULT: '#2563eb', // --primary
					hover: '#1d4ed8',   // --primary-hover
				},
				bg: {
					body: '#f8fafc',    // --bg-body
					card: '#ffffff',    // --bg-card
					hover: '#f1f5f9',   // --bg-hover
				},
				text: {
					primary: '#1e293b', // --text-primary
					secondary: '#64748b', // --text-secondary
				},
				border: {
					DEFAULT: '#e2e8f0', // --border
				},
				success: '#10b981',   // --success
				danger: '#ef4444',    // --danger
				warning: '#f59e0b',   // --warning
				input: {
					bg: '#ffffff'     // --input-bg
				}
			},
			fontFamily: {
				sans: ['Inter', 'sans-serif'],
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
