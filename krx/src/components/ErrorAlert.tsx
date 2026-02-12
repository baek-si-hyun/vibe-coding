"use client";

import { Alert } from "flowbite-react";
import { memo } from "react";

type Props = {
  message: string;
};

function ErrorAlertComponent({ message }: Props) {
  return (
    <Alert color="failure" className="shadow-md">
      <span className="font-medium">{message}</span>
    </Alert>
  );
}

export default memo(ErrorAlertComponent);
