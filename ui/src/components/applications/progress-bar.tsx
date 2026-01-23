interface ProgressBarProps {
  healthyPercentage: number;
  unhealthyPercentage: number;
}

export function ProgressBar({ healthyPercentage, unhealthyPercentage }: ProgressBarProps) {
  return (
    <div className="w-full h-8 bg-gray-200 rounded-lg overflow-hidden flex">
      <div
        className="bg-red-400 flex items-center justify-center text-white text-sm font-semibold"
        style={{ width: `${unhealthyPercentage}%` }}
      >
        {unhealthyPercentage > 0 && `${unhealthyPercentage}%`}
      </div>
      <div
        className="bg-green-400 flex items-center justify-center text-white text-sm font-semibold"
        style={{ width: `${healthyPercentage}%` }}
      >
        {healthyPercentage > 0 && `${healthyPercentage}%`}
      </div>
    </div>
  );
}
