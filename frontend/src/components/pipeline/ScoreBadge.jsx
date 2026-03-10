import { Chip } from '@mui/material';

export const ScoreBadge = ({ score }) => {
    if (score === null || score === undefined) return null;

    let color = 'default';
    let label = '';
    const val = parseFloat(score);

    if (val >= 0.75) {
        color = 'success';
        label = `${val.toFixed(2)} PASS`;
    } else if (val >= 0.5) {
        color = 'warning';
        label = `${val.toFixed(2)} WARN`;
    } else {
        color = 'error';
        label = `${val.toFixed(2)} FAIL`;
    }

    return (
        <Chip
            label={label}
            color={color}
            sx={{
                fontFamily: "inherit",
                fontWeight: 700,
                fontSize: '0.8rem',
                height: 28
            }}
        />
    );
};
