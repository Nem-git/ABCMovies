<?php

declare(strict_types=1);

namespace App\Streaming\Helpers;

final class BentoHelper
{
    //region MP4DUMP

    public static function getMp4Info(
        string $content,
        int $verbosity = 0,
    ): array {
        $filePath = tempnam($_ENV["TEMP_DIR"], "ABC_F_");

        file_put_contents($filePath, $content);

        $cmd = "mp4dump --verbosity $verbosity --format json ";

        $cmd .= $filePath;

        $outputArray = [];

        exec($cmd . " 2>&1", $outputArray);

        // Transform the output array into a string
        // then json parse it to get associative array

        return json_decode(implode("", $outputArray), true);
    }

    //endregion

    //region MP4Edit

    /**
     * @return false|string
     */
    public static function removeAtom(
        string $content,
        array $path,
    ): string|false {
        $filePath = tempnam($_ENV["TEMP_DIR"], "ABC_R_");
        $modifiedFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_M_");

        file_put_contents($filePath, $content);

        $cmd = "mp4edit --remove ";

        $cmd .= join("/", $path);

        $cmd .= " $filePath";
        $cmd .= " $modifiedFilePath";

        exec($cmd . " 2>&1");

        return file_get_contents($modifiedFilePath);
    }

    //endregion

    //region MP4EXTRACT

    //endregion

    // TODO: Add a function to modify an existing atom using offset and new value
    // public static function modifyAtom(string $content)
}
